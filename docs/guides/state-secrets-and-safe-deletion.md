---
page_title: "State, Secrets, and Safe Deletion - Ona Provider"
subcategory: "Security and Secrets"
description: |-
  Understand state storage, write-only values, ephemeral values, and deletion behavior.
---

# State, Secrets, and Safe Deletion

Terraform state is the durable record of managed infrastructure. Treat state and plan files as sensitive, store them in an encrypted remote backend, and restrict access to users who are allowed to inspect Ona resource attributes.

## Sensitive Is Redaction

Terraform `Sensitive` fields are redacted from CLI output and downstream expressions, but they are still stored in Terraform state when they are normal managed resource attributes. Redaction is not encryption and does not remove the value from state.

Use a remote backend with encryption at rest and access controls. Do not write local `terraform.tfstate`, plan files, tokens, private keys, or provider override files into source control.

## Values Not Stored in Plan or State

Write-only arguments are sent to Ona and are not stored in Terraform plan or state. The provider currently uses write-only arguments for:

- `ona_secret.value`
- `ona_scm_integration.oauth_client_secret`
- `ona_runner_llm_integration.api_key`
- `ona_sso_configuration.client_secret`

Ephemeral resources produce values that are not stored in Terraform plan or state. The provider currently exposes:

- `ona_runner_token.token`
- `ona_service_account_token.token`
- `ona_webhook_secret.secret`

Only pass ephemeral values to Terraform ephemeral contexts, write-only arguments, or child module ephemeral outputs.

## Rotation Markers

Write-only arguments cannot produce diffs by themselves because Terraform does not keep their prior value. Rotate write-only values by changing the corresponding stored marker:

- `value_version` for `ona_secret.value`
- `oauth_client_secret_version` for `ona_scm_integration.oauth_client_secret`
- `api_key_version` for `ona_runner_llm_integration.api_key`
- `client_secret_version` for `ona_sso_configuration.client_secret`
- `secret_version` for `ona_webhook`, which rotates the generated signing secret

Changing a secret value without changing its rotation marker can leave the remote value unchanged.

## Delete Versus State Removal

Removing a managed resource block from configuration normally plans a destroy and can delete or disable the remote Ona object. Use `terraform state rm` only when you want Terraform to stop managing an existing remote object without changing it remotely.

Most provider resources delete the remote object on destroy. Review the generated schema for each resource before applying a destroy plan.

Some resources have special destroy behavior:

- `ona_environment_class` disables the remote environment class and removes it from state because the Ona API does not expose deletion.
- `ona_project_insights` disables Insights for the project.
- `ona_announcement_banner` disables and clears the remote banner.
- `ona_terms_of_service` disables the requirement but does not delete immutable version history.
- `ona_organization_policies` restores the server-defined policy configuration captured before Terraform first managed it, then removes the resource from state.
- `ona_oidc_config` removes Terraform state only and does not reset remote organization settings.
- `ona_webhook` deletes the webhook and converts triggers on bound workflows to manual triggers.

Review every destroy plan for these behaviors before applying it.

## Plan and State Expectations

Use `terraform plan` as a review artifact, but do not store plan files in long-lived or broadly accessible locations. A plan can contain enough information to reveal sensitive infrastructure details even when Terraform redacts marked sensitive values.

Prefer a remote backend with locking for shared workflows. Do not run simultaneous applies against the same Ona organization from separate state files unless you have intentionally split ownership.
