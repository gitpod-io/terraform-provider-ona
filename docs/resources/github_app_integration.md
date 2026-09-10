---
page_title: "ona_github_app_integration Resource - ona"
subcategory: "Integrations and Automation"
description: |-
  Manages an organization-owned GitHub App integration. Create and credential rotation require the complete write-only credential bundle and a positive credentials_version. Import and refresh use saved non-secret metadata. Deleting this resource removes the Ona integration; it does not uninstall or delete the GitHub App.
---

# ona_github_app_integration (Resource)

Manages an organization-owned GitHub App integration. Create and credential rotation require the complete write-only credential bundle and a positive credentials_version. Import and refresh use saved non-secret metadata. Deleting this resource removes the Ona integration; it does not uninstall or delete the GitHub App.

Use this resource with an existing organization-owned GitHub App. A GitHub organization administrator registers the App and configures its permissions, event subscriptions, and repository access. Terraform creates the Ona integration for that App. Ona discovers the App slug and OAuth client ID, and the provider resolves the backing integration definition internally.

Use Terraform CLI `>= 1.14`. The dashboard's **Custom GitHub App** workflow remains in preview. Organization administrators can access it when the feature is enabled or the organization already has an integration identified as a custom GitHub App.

## Configure and connect the App

1. A GitHub organization administrator [registers a GitHub App](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/registering-a-github-app), selects its requested permissions and webhook events, and obtains its App ID, PEM private key, OAuth client secret, and webhook secret.
2. Apply this resource with the App ID, complete credential bundle, and a positive `credentials_version`. `enabled` defaults to `true`. Ona saves the integration and discovers `app_slug` and `client_id`.
3. In the GitHub App settings, set the **Callback URL** to `setup.callback_url`, enable **Request user authorization (OAuth) during installation**, and set the **Webhook URL** to `setup.webhook_url`. Set the webhook secret to the same value supplied to Ona. GitHub disables **Setup URL** when OAuth during installation is enabled.
4. An Ona organization administrator opens `setup.installation_url` in an authenticated browser, selects the same Ona organization that Terraform manages, and starts the App's installation flow. A GitHub user authorized to approve the installation selects its repositories and grants the requested permissions. The browser flow connects a new or existing installation to Ona; if another person must approve it, return to Ona and complete the handoff afterward.
5. Refresh Terraform state after the browser flow. `installation` is `null` while unlinked; afterward it contains the saved installation ID, account name, and account type.

`setup.installation_url` is the stable `/settings/org-integrations` entry point on the configured Ona deployment. The callback comes from Ona's canonical GitHub definition, which can use a different host from a custom dashboard domain. Use the returned URLs unchanged: the authenticated browser flow handles the transfer between domains.

Terraform automates integration creation and credential rotation. Initial GitHub authorization and repository selection happen in the browser. Repository scopes and workload eligibility remain governed by the GitHub installation and the existing Ona workload configuration. A refresh reads saved installation metadata; it does not validate access against GitHub.

## Example Usage

Supply `github_app_credentials` through a secret store or another runtime input, and set `github_app_credentials_version` to `1` for creation. The example leaves both variables unset by default so it can also be used for import. The ephemeral sensitive variable and write-only resource argument keep the credential bundle out of Terraform plan and state. See [Terraform sensitive data handling](https://developer.hashicorp.com/terraform/language/manage-sensitive-data).

```terraform
variable "github_app_id" {
  type        = string
  description = "App ID of an existing organization-owned GitHub App."
}

variable "github_app_credentials" {
  type = object({
    private_key    = string
    client_secret  = string
    webhook_secret = string
  })
  description = "Complete credential bundle for creation or rotation. Omit when importing."
  sensitive   = true
  ephemeral   = true
  default     = null
}

variable "github_app_credentials_version" {
  type        = number
  description = "Set to 1 for creation; change to a new positive integer with the full bundle for rotation. Omit when importing."
  default     = null
}

resource "ona_github_app_integration" "company" {
  app_id              = var.github_app_id
  credentials         = var.github_app_credentials
  credentials_version = var.github_app_credentials_version
}

output "github_app_integration_id" {
  value = ona_github_app_integration.company.id
}

output "github_app_setup" {
  value = ona_github_app_integration.company.setup
}

output "github_app_installation" {
  value = ona_github_app_integration.company.installation
}
```

## Rotate credentials

Supply all three credential fields and change `credentials_version` to a new positive integer. A change to a secret alone does not request rotation because Terraform cannot compare write-only values. The version is a local Terraform marker; Ona cannot recover it or detect secret drift during refresh.

Coordinate the GitHub and Ona changes, including the webhook secret on both sides. Rotation for the same `app_id` preserves the Ona integration ID and installation binding. Already-issued GitHub installation tokens may remain valid until they expire.

Changing `app_id` replaces the Ona integration and requires a browser installation handoff for the replacement App. Deleting the resource removes the Ona integration and leaves the GitHub App and its GitHub installation untouched.

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `app_id` (String) GitHub App ID as a canonical positive decimal integer. Changing it replaces the integration.

### Optional

> **NOTE**: [Write-only arguments](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) are supported in Terraform 1.11 and later.

- `credentials` (Attributes, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) Full GitHub App credential bundle. Read only from configuration, never stored in plan or state. Required on creation and whenever credentials_version changes. An empty object is treated as omitted for import and unchanged credentials. Changing only these values does not trigger rotation. (see [below for nested schema](#nestedatt--credentials))
- `credentials_version` (Number) Positive local rotation marker. Change it with the complete credentials bundle to rotate credentials in place. Omit when importing; Ona cannot recover this marker or detect secret drift.
- `enabled` (Boolean) Whether the integration is enabled. Defaults to true.

### Read-Only

- `app_slug` (String) Saved GitHub App slug, refreshed during credential rotation.
- `client_id` (String) Saved GitHub App OAuth client ID.
- `id` (String) Ona integration UUID.
- `installation` (Attributes) Saved GitHub installation metadata, or null while installation is pending. Refresh does not validate installation access with GitHub. (see [below for nested schema](#nestedatt--installation))
- `setup` (Attributes) Stable setup URLs. Complete App installation in the Ona dashboard using an authenticated browser. (see [below for nested schema](#nestedatt--setup))

<a id="nestedatt--credentials"></a>
### Nested Schema for `credentials`

Optional:

- `client_secret` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) GitHub App OAuth client secret.
- `private_key` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) PEM-encoded GitHub App private key.
- `webhook_secret` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) GitHub App webhook secret.


<a id="nestedatt--installation"></a>
### Nested Schema for `installation`

Read-Only:

- `account_name` (String) GitHub account or organization login.
- `account_type` (String) GitHub account kind.
- `id` (String) GitHub installation ID.


<a id="nestedatt--setup"></a>
### Nested Schema for `setup`

Read-Only:

- `callback_url` (String) Canonical OAuth callback URL from the GitHub definition.
- `installation_url` (String) Organization integrations dashboard for the configured Ona deployment.
- `webhook_url` (String) GitHub App webhook endpoint at the canonical callback origin.

## App classification

The provider uses the dashboard's metadata comparison: the integration must reference Ona's canonical `github.com` App definition, both must contain App metadata, and at least one of `app_id`, `app_slug`, or `client_id` must differ. Values are compared exactly. Pending installations and disabled integrations remain eligible.

If all three values match, the integration is classified as the shared App, even when its credentials were configured locally. This heuristic does not establish ownership or grant permissions. Metadata changes can change the classification; if a managed integration no longer qualifies, refresh reports an error and retains its existing Terraform state and ID.

## Import

Import an existing organization-managed GitHub App integration by its Ona integration UUID. Read, import, and Query use saved non-secret metadata and do not require App credentials or authenticate to GitHub. Omit both `credentials` and `credentials_version` during adoption to avoid requesting rotation, and match the existing `app_id` and `enabled` values.

For example, import an enabled integration with structured identity:

```terraform
resource "ona_github_app_integration" "company" {
  app_id = "123456"
}

import {
  to = ona_github_app_integration.company
  identity = {
    integration_id = "00000000-0000-0000-0000-000000000000"
  }
}
```

Replace the example App ID and integration UUID with the existing values. Set `enabled = false` if the integration is disabled. The `integration_id` identity is the Ona UUID, not the numeric GitHub App or installation ID.

<!-- schema generated by tfplugindocs -->
### Identity Schema

#### Required

- `integration_id` (String) Ona integration UUID.

The [`terraform import` command](https://developer.hashicorp.com/terraform/cli/commands/import) also accepts the string UUID:

```shell
#!/usr/bin/env sh

# Configure the existing App ID and leave credentials and credentials_version unset.
terraform import ona_github_app_integration.company 00000000-0000-0000-0000-000000000000
```

## Adopt from `ona_integration`

Keep one Terraform resource address responsible for each remote integration. The generic integration resource and Query can still find the same object; do not manage it at both addresses.

Back up state, record the existing Ona integration UUID, and replace the old `ona_integration` configuration with an `ona_github_app_integration` block matching its App ID and enabled state. Omit credentials and the rotation marker. Then remove only the old state binding and import the same remote integration at the new address:

```shell
umask 077
terraform state pull > before-github-app-adoption.tfstate
terraform state rm ona_integration.company
terraform import ona_github_app_integration.company '<ona-integration-uuid>'
terraform plan
```

Complete the state removal and import before applying configuration. `state rm` leaves the remote integration intact. The follow-up plan should not create, replace, rotate, or delete the integration. This handoff uses state removal and import; cross-type `moved` blocks are not supported.
