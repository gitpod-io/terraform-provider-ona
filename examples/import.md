# Post-process Terraform Query Imports

Use Terraform Query to discover existing Ona resources and generate the
`resource` and identity-based `import` blocks needed to manage them. The helper
in `./scripts` is an optional staging tool that runs Query with the provider from
the current checkout, rewrites unambiguous ID literals to Terraform references,
splits the generated blocks into files, and verifies that the import plan does
not propose remote mutations.

The helper does not discover Ona resources through its own API client and does
not apply the imports. Resource selection and filtering belong in the supplied
`.tfquery.hcl` file.

## Requirements

- Terraform CLI 1.14 or later.
- An Ona API token in `ONA_TOKEN` or passed with `-token`.
- A query configuration containing `list` blocks supported by the provider
  checkout you are using.

List-resource support is incremental. A query can only use resource types
registered by `OnaProvider.ListResources` in that checkout.

## Write a Query

Create `query.tfquery.hcl`. This example discovers every importable runner and
SCM integration:

```hcl
list "ona_runner" "all" {
  provider         = ona
  include_resource = true

  config {
    runner_providers = ["aws_ec2", "gcp"]
  }
}

list "ona_scm_integration" "all" {
  provider         = ona
  include_resource = true
}
```

Add provider-defined filters to narrow the results. For example, runner queries
support `creator_ids` and `runner_providers`; SCM integration queries support
`runner_ids`, `scm_providers`, `hosts`, and `auth_modes`.

## Generate and Validate Configuration

Run the helper from the repository root:

```shell
export ONA_TOKEN="<personal-access-token>"

go run ./scripts \
  -query-file ./query.tfquery.hcl \
  -workdir ./ona-terraform-import
```

The helper performs these steps:

1. Creates a disposable staging directory and copies the query to
   `query.tfquery.hcl`.
2. Builds the provider from `-provider-dir` and configures a Terraform
   development override.
3. Runs `terraform query -generate-config-out=generated.tf`.
4. Saves the unmodified output as `generated.raw.tf.txt`.
5. Derives references from Query-generated identity import blocks and rewrites
   matching string literals only inside generated resource blocks.
6. Splits import and resource blocks into separate `.tf` files.
7. Runs `terraform fmt` and `terraform validate`.
8. Runs a plan and fails if Terraform proposes a remote create, update,
   replacement, or deletion.

The validation plan may contain import actions. Imports associate existing
remote objects with Terraform state; they must not modify those objects.

## Flags

| Flag | Default | Description |
| --- | --- | --- |
| `-query-file` | unset | Path to the query configuration. Required. |
| `-host` | `ONA_HOST` or `https://app.gitpod.io` | Ona application host. |
| `-token` | `ONA_TOKEN` | Ona API token. Required if `ONA_TOKEN` is unset. |
| `-workdir` | `ona-terraform-import` | Disposable staging directory for generated files. |
| `-provider-dir` | `.` | Provider source directory to build and query. |
| `-terraform` | `terraform` | Terraform executable. |
| `-terraform-parallelism` | `2` | Parallelism for the validation plan. |
| `-skip-validate` | `false` | Skip `terraform validate` and the no-mutation plan check. |

## Output

The staging directory contains:

- `query.tfquery.hcl`: the staged query source.
- `versions.tf` and `provider.tf`: minimal Query configuration.
- `imports.tf`: identity-based import blocks emitted by Terraform Query.
- resource files such as `runners.tf`; when present in Query output,
  environment classes and projects use `environment_classes.tf` and
  `projects.tf`, while other resource types use a stable type-derived filename.
- `generated.raw.tf.txt`: Terraform Query output before reference rewriting.
- `generated.rewritten.tf.txt`: rewritten output before splitting.
- `terraformrc` and `.bin/`: the local provider development override.

The helper refuses to clean a work directory containing `terraform.tfstate` or
`terraform.tfstate.backup`. Use the directory only for generation and
validation, not as the long-lived Terraform workspace.

## Review Reference Rewrites

Terraform Query emits an identity object for every discovered resource. The
post-processor uses identities containing exactly one non-null string value as
reference candidates when the identity attribute is `id` or matches the
resource type, such as `runner_id` for `ona_runner`. For example:

```hcl
runner_id = "<runner-id>"
```

can become:

```hcl
runner_id = ona_runner.example.id
```

Composite identities, organization-scoped singleton identities, and duplicate
identity values are intentionally left as literals because rewriting them would
be ambiguous. Import blocks are never rewritten. Review all generated
references before applying the configuration.

## Import into the Target Workspace

Copy `imports.tf` and the reviewed resource files into the real Terraform
workspace. Reconcile its existing provider and version constraints instead of
copying the staging scaffold blindly. Then run:

```shell
terraform plan
terraform apply
terraform plan
```

The first plan should contain imports without remote mutations. Running
`terraform apply` writes the imported resources to state, and the final plan
should be empty. Keep or remove the import blocks after a successful apply
according to the workspace's conventions.

Removing an imported resource block from configuration can cause Terraform to
delete the remote Ona object. To stop managing an object without deleting it,
remove its address from Terraform state before removing its configuration.
