---
page_title: "Authentication and Permissions - Ona Provider"
subcategory: "Identity and Access"
description: |-
  Configure API credentials and permissions for Terraform runs.
---

# Authentication and Permissions

Configure the provider with `ONA_TOKEN` unless a specific workflow needs an explicit `token` argument. Do not commit API tokens to Terraform configuration, generated docs, examples, shell history, or state files.

```shell
export ONA_TOKEN="<api-token>"
```

```hcl
provider "ona" {}
```

## Personal Access Tokens

Use a personal access token (PAT) for initial setup, bootstrap, and Terraform write workflows. Ona documents PATs for CLI and SDK access, and the provider uses the same bearer token path for API calls.

Create the PAT for the human or administrator identity that should own the Terraform operation. Review the plan before apply because managed resources can create, update, disable, or delete remote Ona objects.

Official documentation: [Personal access tokens](https://ona.com/docs/ona/integrations/personal-access-token).

## Service Account Tokens

The provider accepts a service-account token anywhere it accepts `ONA_TOKEN`, but the supported product scope is narrower than a PAT. The official Ona documentation states that service-account tokens can start automations and perform API read operations, and says to contact Ona for additional use cases.

For Terraform reads, use service-account tokens only where the service account has the required read permissions. For Terraform writes, use a PAT unless Ona has confirmed service-account-token write support for your organization and the resources you manage.

Bootstrap and rotate service-account tokens with a user or administrator token. Repository guidance and provider schema both require this conservative workflow because service-account-token-to-service-account-token management is not supported.

Official documentation: [Service accounts](https://ona.com/docs/ona/organizations/service-accounts).

## Permissions

Read-only operations include data sources and Terraform Query list resources. They still require token permissions to read the requested organization, project, runner, warm pool, or security-policy data.

Write operations include managed resources and ephemeral resources that issue tokens or retrieve webhook signing secrets. They require the corresponding Ona product permission, such as project administration, runner administration, group administration, organization administration, or webhook update permission.

For organization-level settings, service accounts need access through groups and organization roles. Manage custom groups with `ona_group`, add service accounts with `ona_group_membership`, and grant roles with `ona_organization_role_assignment`.

Official documentation: [Manage groups](https://ona.com/docs/ona/organizations/groups) and [Organization roles](https://ona.com/docs/ona/organizations/organization-roles).

## Host Configuration

Omit `host` for the default Ona host. Set `ONA_HOST` or the provider `host` argument only when your organization uses a non-default Ona application host.

```shell
export ONA_HOST="https://<ona-hostname>"
```
