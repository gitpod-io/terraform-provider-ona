# Terraform-Native Import of Existing Ona Resources

Brownfield import should be a Terraform-native workflow. An organization may start by managing resources in the Ona UI, then later move to Terraform for reviewable, repeatable, code-driven management.

The target workflow is:

1. Terraform asks `terraform-provider-ona` to discover existing resources.
2. Terraform import blocks connect those remote resources to Terraform addresses.
3. Terraform config generation writes starter HCL.
4. Terraform imports the resources into state.
5. `terraform plan` proves the imported configuration is a no-op against production.

## Current Status

The provider includes native resources for projects, runner registrations,
runner environment classes, runner SCM integrations, security policies, and
organization policies, announcement banners, Terms of Service, groups, group
runner environment classes, runner SCM integrations, security policies,
organization policies, announcement banners, Terms of Service, custom domains,
groups, group memberships, and organization role assignments. Terraform can
create, read, update, delete where the Ona API supports deletion, and import
those resource types directly.

Terraform cannot discover or import a resource type natively until each resource has:

- `ImportState` support,
- a `Read` implementation that fully refreshes Terraform state from the Ona API,
- resource identity where stable identity fields exist, and
- list-resource support for discoverable resource families.

Until those provider primitives exist for the rest of the resource graph, the
helper in `./scripts` prepares Terraform-native import configuration only for
registered/importable provider resources. It still discovers the broader
resource graph for inventory and future reference rewriting, but it writes
import blocks only for resource types enabled in the helper's selection path,
which currently includes project, runner, and environment class resources.
Security policies, organization policies, custom domains, groups, group
memberships, and organization role assignments are provider-native resources,
but the helper does not yet select them for generated import blocks.

Direct `terraform import` uses these resource IDs:

| Resource | Import ID |
| --- | --- |
| `ona_runner` | Runner ID |
| `ona_scm_integration` | SCM integration ID |
| `ona_environment_class` | Environment class ID |
| `ona_project` | Project ID |
| `ona_security_policy` | Security policy ID |
| `ona_organization_policies` | `current` or the authenticated organization ID |
| `ona_announcement_banner` | `current` |
| `ona_terms_of_service` | `current` |
| `ona_custom_domain` | `current` |
| `ona_group` | Group ID |
| `ona_group_membership` | `group_id/service_account_id` |
| `ona_organization_role_assignment` | `group_id/organization_id/role` |
| `ona_webhook` | Webhook ID |
| `ona_integration` | Integration ID |
| `ona_workflow` | Workflow ID |

Importing `ona_integration` restores API-observable configuration, but Ona
censors stored credentials in read responses. Terraform therefore leaves the
write-only `credentials` object and its version markers unset after import. To
rotate an imported credential, configure the credential value and its matching
version marker, review whether Terraform proposes an update or replacement,
then apply.

## Workflow Import

Add an import block with the workflow ID:

```hcl
import {
  to = ona_workflow.nightly_checks
  id = "00000000-0000-0000-0000-000000000000"
}
```

Run `terraform plan -generate-config-out=generated.tf`, review the generated
configuration, apply the import, and run `terraform plan` again. The final plan
should be empty. Existing workflows that use reports, report steps,
workflow-level agent/Codex settings, or legacy pull-request triggers without a
webhook or integration must be updated in Ona before import because
`ona_workflow` cannot reproduce those fields.

Removing an imported `ona_workflow` from configuration deletes it remotely.
Remove its address from Terraform state instead when you only want Terraform to
stop managing it.

## Native Runner Import

For the Runner-only dogfood slice, use Terraform import blocks directly:

```hcl
terraform {
  required_providers {
    ona = {
      source  = "gitpod-io/ona"
      version = "~> 0.1"
    }
  }
}

provider "ona" {}

resource "ona_runner" "frankfurt" {}

import {
  to = ona_runner.frankfurt
  id = "01980ed3-a090-7b5b-a74c-9bf5d8cfe53c"
}
```

Then run:

```shell
export ONA_TOKEN="<service-account-or-personal-access-token>"
terraform init
terraform plan -generate-config-out=generated.tf
terraform apply
terraform plan
```

The final `terraform plan` should be a no-op. After import, `ona_runner`
manages the remote runner registration lifecycle. Updating the Terraform
resource updates the runner registration, and removing the resource from
configuration calls the Ona API to delete the runner. To stop managing an
existing runner without deleting it, remove it from Terraform state instead of
applying a resource deletion.

Run the helper from the repository root:

```shell
ONA_TOKEN="<personal-access-token>" go run ./scripts
```

The helper logs each step to stderr with timestamps.

API calls use Ona's public API endpoints directly. Rate-limited API requests are retried up to 20 times with a maximum retry delay of 30 seconds. Terraform's own read concurrency is controlled by `-terraform-parallelism`.

## Flags

| Flag | Default | Description |
| --- | --- | --- |
| `-host` | `ONA_HOST` or `https://app.gitpod.io` | Ona host. |
| `-token` | `ONA_TOKEN` | Personal access token. Required if `ONA_TOKEN` is unset. |
| `-org-id` | Authenticated token organization | Organization to discover. |
| `-workdir` | `ona-terraform-import` | Output directory. |
| `-provider-dir` | `.` | Provider source directory to build for Terraform's development override. |
| `-terraform` | `terraform` | Terraform executable. |
| `-resource-type` | unset | Resource type to import. Repeat or comma-separate values. |
| `-resource-kind` | unset | Alias for `-resource-type`. |
| `-resource-id` | unset | Resource UUID or import ID to import. Repeat or comma-separate values. |
| `-terraform-parallelism` | `2` | Terraform plan parallelism for API reads. |
| `-include-system-groups` | `false` | Include system-managed and direct-share groups. |
| `-skip-terraform` | `false` | Stop after writing the import map, provider scaffold, references, and import blocks. |
| `-skip-validate` | `false` | Skip `terraform validate` and the production no-op plan check. |
| `-refresh-import-map` | `false` | Ignore an existing `import-map.json` and rediscover resources. |

## Examples

Import one runner:

```shell
ONA_TOKEN="<personal-access-token>" go run ./scripts \
  -resource-type runner \
  -resource-id 01980ed3-a090-7b5b-a74c-9bf5d8cfe53c \
  -workdir ona-terraform-import-frankfurt-runner
```

Import all runners:

```shell
ONA_TOKEN="<personal-access-token>" go run ./scripts \
  -resource-type runner \
  -workdir ona-terraform-import-runners
```

## Terraform Workflow

The helper prepares and validates the Terraform workflow in these stages:

1. Resolve the organization from `-org-id` or `GetAuthenticatedIdentity`.
2. Build or reuse `import-map.json`.
3. Select the resources requested by `-resource-type` and `-resource-id`.
4. Add reference resources needed by the selection.
5. Write `versions.tf`, `provider.tf`, `references.tf`, and `imports.tf`.
6. Build the local provider binary into `<workdir>/.bin/terraform-provider-ona`.
7. Write `terraformrc` with a provider development override for `registry.terraform.io/gitpod-io/ona`.
8. Run `terraform plan -generate-config-out=generated.tf`.
9. Save the raw generated file as `generated.raw.tf.txt`.
10. Rewrite known UUID literals to Terraform references.
11. Split generated resources into idiomatic files.
12. Run `terraform fmt`.
13. Run `terraform validate`, unless `-skip-validate` is set.
14. Run a production plan and fail if Terraform proposes remote create, update, replace, or delete actions.

## Import Map

`import-map.json` is the source of truth for resource selection and reference rewriting. It contains:

- `organization_id`: discovered or configured organization ID.
- `resources`: importable resources with `type`, Terraform `address`, `uuid`, `import_id`, `name`, `references`, `reference_ids`, and optional `skip_reason`.

The discovery pass lists:

- groups and group memberships
- teams
- security policies
- runners
- runner environment classes
- projects and project environment classes
- service accounts
- automations
- organization policies

By default, group discovery excludes system-managed and direct-share groups. Set `-include-system-groups` to include them.

The helper reuses an existing `import-map.json` in the output directory. When reusing the import map, it augments referenced objects that are needed for rewrites, such as runner environment classes and service accounts. Use `-refresh-import-map` to discard the existing import map and rediscover resources.

## Selection

Use `-resource-type` or `-resource-kind` to select resource kinds. Values can be Terraform type names or short names. The helper currently supports these selected import types:

- `runner` or `ona_runner`
- `environment-class` or `ona_environment_class`
- `project` or `ona_project`

Use `-resource-id` to narrow the selection by UUID or import ID. The selector applies an intersection: when both type and ID are set, a resource must match both.

The helper automatically adds selected dependencies used by generated references.

The helper does not currently select other discovered resources for generated
import blocks, including groups, group memberships, organization role
assignments, announcement banners, Terms of Service, teams, security policies,
organization policies, automations, or AI budget policies. Security policies,
organization policies, announcement banners, Terms of Service, groups, group
memberships, and organization role assignments can still be imported directly
with Terraform import blocks because the provider now implements those
resources.
import blocks, including custom domains, groups, group memberships,
organization role assignments, announcement banners, Terms of Service, teams,
security policies, organization policies, automations, or AI budget policies.
Security policies, organization policies, announcement banners, Terms of
Service, custom domains, groups, group memberships, and organization role
assignments can still be imported directly with Terraform import blocks because
the provider now implements those resources.

## Output Files

By default, output is written to `ona-terraform-import/`. The directory contains:

- `import-map.json`: discovered resources and references. The helper reuses this file on later runs unless `-refresh-import-map` is set.
- `versions.tf`: provider requirements.
- `provider.tf`: provider configuration.
- `imports.tf`: Terraform import blocks.
- `references.tf`: local reference objects for resources that Terraform cannot import directly, such as service accounts.
- `generated.raw.tf.txt`: raw Terraform-generated configuration before rewrite.
- `generated.rewritten.tf.txt`: rewritten generated configuration before splitting.
- `<resources>.tf`: split resource files.
- `terraformrc` and `.bin/`: local provider development override.
- `terraform.sh`: wrapper that runs Terraform with the generated CLI configuration.

Before writing files, the command cleans stale Terraform outputs in the work directory. It preserves `import-map.json` and unrelated directories, then removes old generated `.tf`, `.tf.txt`, state, plan, lock, wrapper, Terraform CLI configuration files, and known generated directories such as `.terraform` and `.bin`.

Generated resources are split by type as more provider resources are added:

- `automations.tf`
- `groups.tf`
- `organization_policies.tf`
- `projects.tf`
- `environment_classes.tf`
- `runners.tf`
- `security_policies.tf`
- `teams.tf`
- `generated_misc.tf` for any block the splitter does not recognize

## Reference Rewrites

The helper rewrites UUIDs only when the import map contains a safe reference relationship.

- Project environment class IDs can become `ona_environment_class.<name>.id`.
- Project prebuild executor IDs can become `local.service_accounts.<name>.id`.
- Runner environment class `runner_id` values can become `ona_runner.<name>.id`.
- Group service-account member IDs can become `local.service_accounts.<name>.id`.

User IDs remain raw IDs because users are not Terraform resources in this provider.

## Terraform Validation

The default helper run builds the provider from `.`, configures a Terraform development override in the output directory, runs native config generation, rewrites and splits generated HCL, formats and validates the generated configuration, then runs a production plan. The run fails if the final plan proposes create, update, replace, or delete actions.

Use `-skip-terraform` to stop after writing `import-map.json`, `versions.tf`, `provider.tf`, `references.tf`, and `imports.tf`.

Use `-skip-validate` to skip `terraform validate` and the production no-op plan check. This is useful while debugging provider schema issues, but do not treat the generated files as ready to apply until validation passes.

Use `-terraform-parallelism` to control Terraform API read concurrency during config generation and validation. The default is `2`.

## Provider Requirements

Terraform-native import depends on provider support. Each importable Ona resource should implement `ImportState`, refresh complete durable state in `Read`, expose resource identity where stable identity fields exist, and provide list-resource support when the Ona API can enumerate that resource family.

As those provider capabilities are added, the helper should use them rather than duplicating discovery and import behavior in repository code.

## Terraform Wrapper

After successful config generation, run Terraform through the wrapper in the generated directory:

```shell
ONA_TOKEN="<personal-access-token>" ./ona-terraform-import/terraform.sh plan
ONA_TOKEN="<personal-access-token>" ./ona-terraform-import/terraform.sh apply
```

The wrapper sets `TF_CLI_CONFIG_FILE` to the generated `terraformrc`, then runs Terraform with `-chdir=<workdir>`. It requires `ONA_TOKEN` in the environment.

If the configuration changes after a saved plan is created, discard the saved plan and run `plan` again. Terraform saved plans are tied to the exact provider selections and configuration used to create them.

## Script Layout

The helper is split by responsibility:

- `main.go`: flag parsing, API client setup, high-level orchestration, live log output.
- `types.go`: config, import map, and discovery snapshot types.
- `collect.go`: API discovery for organization resources and referenced external objects.
- `inventory.go`: import map construction, Terraform address generation, label generation, reference tracking, skip reasons, and merging.
- `selection.go`: resource type normalization, resource ID filtering, and automatic reference-resource inclusion.
- `files.go`: output directory cleanup, Terraform scaffold files, import blocks, reference locals, and JSON writing.
- `terraform.go`: local provider build, Terraform CLI development override, wrapper script, plan execution, and no-op plan validation.
- `rewrite.go`: HCL parsing, UUID-to-reference rewrites, multiline tuple formatting, and generated file splitting.

## Development Notes

The helper deliberately uses Terraform native config generation instead of hand-rendering resource HCL. Hand-written logic is limited to pieces Terraform cannot infer:

- discovering which organization resources exist
- deciding which resources should be imported
- creating import blocks
- providing local reference objects for non-resource objects such as service accounts
- rewriting raw UUIDs to Terraform references
- splitting generated HCL into stable files
- proving that the generated configuration is a no-op against production

This keeps provider schema interpretation inside Terraform while keeping Ona-specific graph knowledge in the helper.
