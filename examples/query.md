# Terraform Query for Existing Ona Resources

Terraform Query discovers existing Ona resources through provider list resources. Use it to inspect importable resources and to generate starter Terraform configuration before deciding what to import or manage.

The query example requires Terraform 1.14 or later.

## Runner Query

This example uses `ona_runner` to discover existing runner registrations and write generated configuration:

```shell
export ONA_TOKEN="<service-account-or-personal-access-token>"
export ONA_HOST="${ONA_HOST:-https://app.gitpod.io}"

./examples/resources/ona_runner/query.sh
```

The script builds the provider from the current repository, writes a temporary Terraform CLI development override, runs `terraform query -generate-config-out=generated.tf`, and prints the generated file path. By default, the output stays in a `mktemp` directory so the repository is not modified. Pass an output path when you want to copy the generated file somewhere specific. It does not write Terraform state.

The query source lists importable AWS EC2 and GCP runners:

```hcl
list "ona_runner" "all" {
  provider         = ona
  include_resource = true

  config {
    runner_providers = ["aws_ec2", "gcp"]
  }
}
```

`ona_runner` also accepts `creator_ids` when you want to limit discovery to runners created by specific subject IDs.

Set `include_resource = true` when you want Terraform to generate resource configuration. Without it, Terraform can list identities and display names, but it does not have full resource values to emit as HCL.

For the full local-provider flow, see [query.sh](resources/ona_runner/query.sh).

## Output

`terraform query -generate-config-out=generated.tf` writes Terraform resource configuration for the discovered runners. The generated file is a starting point. Review it before applying, rename resource labels as needed, and keep it with import blocks when moving existing resources under Terraform management.

Query does not import resources into Terraform state. To manage a discovered runner after reviewing the generated configuration, use Terraform import blocks or the import helper described in [import.md](import.md).

## Provider Requirements

Terraform Query depends on provider list-resource support. A queryable Ona resource needs a list resource, stable resource identity, import support, and a `Read` implementation that can refresh complete durable state from the Ona API.

As more Ona resources gain list-resource support, add query examples that use provider-native discovery rather than duplicating discovery logic in scripts.
