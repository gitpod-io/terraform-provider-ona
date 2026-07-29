# Local Import Loop

This example exercises every resource type currently registered for Terraform
Query and imports the discovered objects into disposable local Terraform state.
It covers:

- `ona_runner`
- `ona_scm_integration`
- `ona_environment_class`
- `ona_project`
- `ona_warm_pool`

The example only generates imports for objects that already exist and can be
represented by the provider. To exercise the complete workflow, use an Ona
organization containing at least one importable object of each type.

For a repeatable fixture, apply the
[local development loop](../local-devloop/README.md) first. Its default
configuration creates runners, SCM integrations, an environment class, a
project, and a warm pool. The import loop can also discover other matching
objects in the organization, so review the generated configuration before
applying it.

## Prepare the Local Provider

Terraform 1.14 or later is required. From the repository root, build the
provider and configure a development override:

```shell
mkdir -p .bin
go build -o .bin/terraform-provider-ona .
cat >terraformrc <<EOF
provider_installation {
  dev_overrides {
    "gitpod-io/ona" = "${PWD}/.bin"
  }
  direct {}
}
EOF

export ONA_TOKEN="<service-account-or-personal-access-token>"
export ONA_HOST="${ONA_HOST:-https://app.gitpod.io}"
export TF_CLI_CONFIG_FILE="${PWD}/terraformrc"
```

Do not commit `.bin/`, `terraformrc`, credentials, generated configuration,
plans, or Terraform state.

## Discover and Import Every Supported Type

Run Query to generate resource blocks and identity-based import blocks for all
registered list resources:

```shell
terraform -chdir=dev/local-importloop query \
  -generate-config-out=generated.tf
```

Review the generated configuration, then create and inspect the import plan:

```shell
terraform -chdir=dev/local-importloop plan \
  -input=false \
  -out=import.tfplan
terraform -chdir=dev/local-importloop show import.tfplan
```

Proceed only when the plan contains imports and no remote create, update,
replace, or delete actions. Apply the saved plan and verify that the imported
configuration is stable:

```shell
terraform -chdir=dev/local-importloop apply -input=false import.tfplan
terraform -chdir=dev/local-importloop plan -detailed-exitcode -input=false
```

The final command exits with status `0` when the imported configuration is a
no-op. Status `1` means Terraform failed, while status `2` means Terraform still
proposes changes and the generated configuration or provider mapping needs
investigation.

Terraform Query is read-only. Applying the generated import blocks writes only
the local state in this directory, but that state then represents real remote
objects. When the local development loop supplied the fixtures, both directories
temporarily track the same objects. Never run `terraform destroy` from the
import loop.

Reset the import loop by removing its generated configuration, saved plan, and
state together without running another plan or apply:

```shell
rm -f \
  dev/local-importloop/generated.tf \
  dev/local-importloop/import.tfplan \
  dev/local-importloop/terraform.tfstate \
  dev/local-importloop/terraform.tfstate.backup
```

After removing the import-loop state, the local development loop remains the
sole owner of its fixtures and can destroy them normally.

The list-resource coverage test fails when a future Query-enabled resource is
registered without a matching block in `query.tfquery.hcl`.
