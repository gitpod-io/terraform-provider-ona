# Terraform Provider for Ona

This module contains the in-repository Terraform provider for Ona.

The provider manages Ona projects, runners, runner environment classes, security policies, organization policies, product Automations, teams, groups, and AI budget policies. Product Automations are exposed as `ona_automation` resources and are backed by the Ona Workflow API internally.

## Authentication

Use environment variables for credentials. This is the normal Terraform provider pattern: it keeps tokens out of `.tf` and `.tfvars` files, and lets CI systems or Terraform Cloud inject secrets without committing them to the repository.

```bash
export ONA_TOKEN="<personal-access-token>"
export ONA_HOST="https://app.gitpod.io"
```

The provider also supports a sensitive `token` argument for cases where the value is supplied by a secret manager or CI variable. Do not commit token values in `.tf` or `.tfvars` files; Terraform `sensitive` values are redacted from output, but they are not a secret storage mechanism.

## Example

Project configuration:

```hcl
terraform {
  required_providers {
    ona = {
      source = "registry.terraform.io/ona/ona"
    }
  }
}

provider "ona" {}

resource "ona_project" "example" {
  name = "backend-service"

  initializer = {
    spec = [
      {
        git = {
          remote_uri        = "https://github.com/acme/backend-service"
          target_mode       = "remote_branch"
          clone_target      = "main"
          checkout_location = "backend-service"
        }
      }
    ]
  }
}
```

See `examples/project.tf` for a fuller project sample, `examples/workflow.tf` for a product Automation workflow sample, and `examples/policies.tf` for security and organization policy samples. Generated-style provider documentation lives in `docs/`.

Resources that operate at organization scope use the organization associated with the authenticated token. Terraform configuration does not set organization IDs.

## Export Existing Resources

The export script can discover organization resources, generate Terraform import blocks, run Terraform's native config generation, and split the generated HCL into resource files.

```bash
go run ./frontend/terraform-provider-ona/scripts \
  -resource-type ona_project \
  -resource-id 01900000-0000-7000-8000-000000000000
```

Use `-resource-type` or `-resource-kind` to export every resource of a type, and add one or more `-resource-id` values to narrow that selection by UUID or import ID. Both flags can be repeated or comma-separated.

Runner environment classes are exported as `ona_runner_environment_class` resources. The exporter also uses inventory reference data to rewrite project environment class IDs, runner IDs, and service account IDs to Terraform references where possible.

Import existing policy resources with:

```bash
terraform import ona_security_policy.restricted 0391457c-99ec-7374-9c50-5f51ecc33fc9
terraform import ona_organization_policies.example current
```
