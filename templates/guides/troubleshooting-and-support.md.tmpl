---
page_title: "Troubleshooting and Support - Ona Provider"
subcategory: "Integrations and Automation"
description: |-
  Diagnose provider installation, authentication, permission, host, and secret-rotation issues.
---

# Troubleshooting and Support

Start with `terraform init`, `terraform validate`, and `terraform plan`. Most provider failures are installation, token, permission, host, or rotation-marker problems.

## Provider Installation

Use the Registry source address `gitpod-io/ona` and pin an intentional beta version.

```hcl
terraform {
  required_providers {
    ona = {
      source  = "gitpod-io/ona"
      version = "= 0.2.0-beta.2"
    }
  }
}
```

The current beta release publishes Linux `amd64` and Linux `arm64` artifacts. Terraform installation on macOS or Windows is not supported by the current release artifacts.

If installation fails, confirm the source address, version constraint, Terraform CLI version, and platform. Then remove `.terraform/` and `.terraform.lock.hcl` in a scratch workspace and run `terraform init -upgrade` again.

## Missing or Invalid Credentials

The provider reads `ONA_TOKEN` by default. If configuration fails with a missing token or unauthenticated API error, export a token and rerun the command.

```shell
export ONA_TOKEN="<api-token>"
terraform plan
```

Avoid setting `token` directly in HCL unless your workflow has a specific reason. Never commit real token values.

## Insufficient Permissions

Read failures usually mean the token cannot see the requested organization, project, runner, warm pool, or policy. Write failures usually mean the token lacks the relevant product administration permission.

Use a PAT for bootstrap and write workflows unless Ona has confirmed service-account-token write support for your organization. For service accounts, check group membership and organization role assignment.

Official documentation: [Service accounts](https://ona.com/docs/ona/organizations/service-accounts), [Manage groups](https://ona.com/docs/ona/organizations/groups), and [Organization roles](https://ona.com/docs/ona/organizations/organization-roles).

## Host Configuration Failures

Most users should omit `host`. Set `ONA_HOST` only for a non-default Ona application host and include the scheme, for example `https://<ona-hostname>`.

If the API client cannot configure or every request fails against the host, unset `ONA_HOST` and retry against the default host before investigating network or DNS problems.

## Secret Rotation Problems

Write-only arguments do not create diffs by themselves. When rotating a write-only value, change both the secret value and its rotation marker.

For `ona_webhook`, changing `secret_version` rotates the generated signing secret and immediately invalidates the previous value. Retrieve the current value with `ona_webhook_secret` and pass it only through ephemeral-safe outputs or write-only consumers.

## Reporting Issues

Report provider bugs in the [source repository](https://github.com/gitpod-io/terraform-provider-ona/issues). Include the provider version, Terraform CLI version, platform, resource or data source name, a redacted configuration snippet, and the redacted error output.

For Ona product behavior, account access, organization setup, or feature enablement, use [Ona support](https://ona.com/support). For security issues in the provider repository, use the [security policy](https://github.com/gitpod-io/terraform-provider-ona/security/policy).

Official troubleshooting documentation: [Troubleshooting guide](https://ona.com/docs/ona/troubleshooting).
