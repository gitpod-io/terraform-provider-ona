# Ona provider workflows

Use these provider-specific playbooks after resolving the configured `gitpod-io/ona` version. Verify exact types, arguments, identities, and lifecycle behavior in that version's Registry documentation or installed schema.

## Configure the Ona provider

- Use source `gitpod-io/ona` with an intentional constraint. Preserve an existing compatible pin.
- Supply credentials through `ONA_TOKEN`. Keep `provider "ona" {}` empty for the default host.
- Set `ONA_HOST` or `host` only for a non-default Ona application host and include the scheme. The provider's default application host is `https://app.gitpod.io`.
- Use a PAT for initial setup and write workflows. A service-account token may cover reads and automation starts, but do not assume general write access.
- Remember that the authenticated token selects the organization context used by organization-scoped operations.

For authentication failures, distinguish a missing/invalid token, a host mismatch, and insufficient product permissions before changing configuration.

## Build the runner-to-project graph

Use references to express this order:

```text
ona_runner
  ├─ ona_environment_class
  │    ├─ ona_project.environment_class
  │    └─ ona_warm_pool
  ├─ runner-scoped SCM / LLM configuration
  ├─ runner policy
  └─ ona_runner_token (ephemeral bootstrap output)
```

Apply these Ona-specific rules:

- A runner resource creates the remote runner registration; deployment of the actual cloud runner follows the provider-specific setup output.
- Environment classes belong to a runner. Provider-specific class configuration and runner changes may replace the class.
- A project is backed by a Git repository and must select at least one runner environment class or a local-runner entry, according to the selected schema.
- Project prebuild settings and warm pools are related but separate. Create a warm pool only after prebuilds are enabled for that project and environment class.
- Use `runner_id`/resource references where documented rather than copying IDs between resources.
- Runner-provider changes and infrastructure-shaping fields can cause replacement; inspect the exact plan before changing them.

## Build organization access control

Use references to express this order:

```text
ona_service_account ── ona_group_membership ── ona_group
                                               └─ ona_organization_role_assignment
```

- Group membership connects one service account to one group and is identity-like; changing either side replaces the membership.
- Organization role assignment grants the group an organization role in the authenticated token's organization and is also replacement-oriented.
- Bootstrap service accounts and tokens with a user/admin credential. Do not assume a service-account token can manage service accounts or grant itself permissions.
- Diagnose write failures by checking both group membership and organization role assignment, then the product permission required by the target resource.

## Choose reads, discovery, and ownership

- Use a singular or collection data source only when the selected provider version documents that read surface.
- Use Terraform Query for provider list resources. Query support is narrower than managed-resource support; verify the available list types instead of expecting every Ona object to be discoverable.
- Runner discovery can return identities and, when requested by the documented query configuration, starter resource HCL.
- Query does not write Terraform state and does not make Ona objects managed. Pair reviewed generated HCL with import blocks to establish ownership.
- Confirm that the installed Terraform CLI supports the Query syntax used by the selected provider version. Do not silently upgrade the provider or CLI.

## Import by Ona identity family

Ona imports do not all use the same identity shape. Determine the family from versioned docs:

- **Object identity:** many resources import by the Ona object ID.
- **Organization singleton:** organization-wide settings may import with `current`, the authenticated organization identity, or another documented singleton identity.
- **Composite relationship:** memberships and role assignments may combine related IDs and role values.
- **Scoped object:** Ona secrets encode scope and, when relevant, the owning project, user, or service account in the import identity.

Use a declarative block only after verifying the exact format:

```hcl
import {
  to = ona_<documented_type>.<name>
  id = var.<documented_import_identity>
}
```

Before import, confirm that no other address or state owns the object. Review generated HCL for server defaults, unavailable fields, censored credentials, and lifecycle-sensitive values. Apply the import only with authorization, then require a no-op plan. A non-no-op result means the configuration, identity, version, or provider representation still differs from Ona.

## Handle Ona secrets and credentials

The provider uses two distinct patterns. Verify each named capability in the selected version.

Use the names below to locate the relevant versioned documentation; do not treat this table as a schema or turn it into exact HCL before resolving the consumer's provider and Terraform versions. If those versions are unavailable, explain the pattern with placeholders and stop.

### Write-only inputs with rotation markers

| Ona configuration | Write-only value | Stored rotation marker |
| --- | --- | --- |
| Ona secret | `value` | `value_version` |
| Runner SCM integration | `oauth_client_secret` | `oauth_client_secret_version` |
| Runner LLM integration | `api_key` | `api_key_version` |
| SSO configuration | `client_secret` | `client_secret_version` |
| Runner custom metrics | `password` | `password_version` |

Change the value and marker together. The marker is non-secret state used to force resubmission because Terraform cannot diff a value it never stores. Imported resources cannot recover censored write-only values; leave them unset unless intentionally rotating them.

### Ephemeral outputs

- `ona_runner_token` creates a short-lived runner registration token.
- `ona_service_account_token` returns a service-account token once for bootstrap or rotation.
- `ona_webhook_secret` retrieves the current signing secret; access is audited and requires webhook-update permission.

Send these outputs only through Terraform-supported ephemeral contexts, child-module ephemeral outputs, or documented write-only arguments. If a token must survive the run, write it to an approved external secret manager through a non-persisting path. Never expose it in an ordinary output, local file, provisioner command line, or state-backed argument.

Webhook rotation is driven by the managed webhook's documented secret-version marker; retrieving the ephemeral secret does not itself rotate it. The previous signing secret is invalidated by rotation, so coordinate downstream consumers.

Choose stable, non-secret marker values according to the consumer's convention. Do not invent a date/version value or use an always-changing expression merely to make a plan show a rotation.

## Manage automations carefully

- Use a permitted user credential to create an Ona automation; the Ona API rejects creation by service accounts.
- Check executor ownership before changing triggers or actions. A caller may need to remain the current user executor, select themselves, or select an allowed service account.
- Verify whether the existing automation uses features the selected provider version does not model. Do not import or overwrite unsupported action, report, model, agent, or legacy-trigger configuration.
- Destroy uses graceful deletion: idle automations may disappear immediately, while active executions are cancelled and cleanup completes asynchronously.

## Diagnose Ona provider failures

1. **Provider configuration:** confirm only that `ONA_TOKEN` is present; never print it. Unset an unintended `ONA_HOST` before investigating the default host.
2. **Authentication:** replace invalid or expired credentials through the approved process. Do not paste tokens into diagnostics.
3. **Authorization:** map the operation to project, runner, group, organization, webhook, or automation permissions. For service accounts, inspect group and role assignment rather than granting broad access blindly.
4. **Rotation did nothing:** check that the documented marker changed with the write-only value.
5. **Import drifts:** check identity shape, server defaults, censored fields, unsupported remote features, and the exact provider version.
6. **Unexpected replacement:** inspect runner/provider identity, environment-class configuration, relationship IDs, scopes, and other documented ForceNew fields.

Report provider version, Terraform version, platform, `ona_*` type, redacted configuration, redacted diagnostic, and non-secret request/correlation details. Send provider defects to the provider repository and product/account/feature issues to Ona support.

## Validate without losing the Ona context

- Run `terraform fmt` and `terraform validate` after edits.
- Run `terraform init` when dependency installation is appropriate; review lock-file changes.
- Run a live `terraform plan` only with explicit authorization.
- Interpret the plan in product terms: which runner registration, environment class, project, warm pool, policy, identity assignment, secret, webhook, integration, or automation changes remotely, and whether Ona deletes, disables, clears, restores, or retains anything.
