# Ona provider safety

Read this reference before credentials, secrets, imports, state changes, replacements, or destroys.

## Protect Ona authentication

- Supply the provider token through `ONA_TOKEN`; do not put it in the provider block, `.tfvars`, shell command arguments, generated HCL, logs, or source control.
- Use a PAT for bootstrap and write workflows. Use service-account tokens only for documented read or automation-start behavior unless Ona has confirmed the exact write capability.
- Set `ONA_HOST` only for a non-default Ona application host. An unintended host can make a valid token appear invalid.
- Treat authenticated Query, data-source reads, plans, ephemeral credential issuance, and webhook-secret retrieval as live Ona operations requiring authorization.

Never print a token to test it. Confirm only whether the variable is set and preserve redaction in diagnostics.

## Keep Ona secrets out of state

`sensitive = true` redacts display but does not prevent Terraform state storage. Use the provider's documented write-only arguments and ephemeral resources for non-persistence.

- Change each write-only value with its stored rotation marker. A value change without the marker may not reach Ona.
- Keep ephemeral values inside ephemeral-compatible paths. Do not convert them into ordinary outputs, locals, files, provisioner arguments, or state-backed resource arguments.
- Store an issued service-account token or signing secret only in an approved external secret manager through a documented non-persisting sink.
- Do not inspect raw state to recover a secret. Imported or refreshed resources cannot recover values that Ona censors or returns only once.
- Remember that switching to a write-only argument does not erase secret values from historical state snapshots created by older configurations.

Stop if any step from Ona issuance to the final consumer lacks a documented non-persistence guarantee.

## Understand Ona destroy behavior

Do not translate a Terraform destroy symbol to “remote object deleted” without reading the selected version's resource documentation. Known Ona lifecycle patterns include:

- Environment-class destroy disables the class because Ona does not expose deletion.
- Announcement-banner destroy disables and clears the banner.
- Terms-of-service destroy disables the requirement while retaining immutable version history.
- Organization-policies destroy restores the server configuration captured before Terraform took ownership.
- OIDC-config destroy may remove only Terraform state without resetting remote organization settings.
- Webhook destroy deletes the webhook and can convert bound workflow triggers to manual triggers.
- Automation destroy is graceful and may cancel active executions before asynchronous cleanup finishes.

Verify these behaviors against the pinned provider version. For projects, disable Insights through its setting when that is the intended outcome; do not delete the project merely to disable Insights.

Removing a resource block and applying can invoke these remote behaviors. `terraform state rm` only relinquishes Terraform ownership at that moment. Never conflate the two.

## Review Ona replacements

Treat replacements as potentially disruptive when they affect:

- a runner registration or its cloud deployment;
- an environment class referenced by projects, prebuilds, or warm pools;
- group membership or organization role assignment;
- a secret scope or credential-proxy configuration;
- an integration, webhook, or automation executor/trigger relationship.

Resolve the exact ForceNew field from the matching provider schema. Explain dependent Ona objects, downtime, credential invalidation, and migration order before proceeding.

## Import without overwriting Ona

- Verify whether the identity is a plain object ID, organization singleton, composite relationship, or scoped-secret identity.
- Confirm no other Terraform state owns the object.
- Review generated configuration for Ona server defaults and fields the provider cannot observe.
- Do not populate censored credential fields merely to make generated HCL look complete.
- Require a no-op post-import plan. Any proposed update, replacement, disable, or deletion means the migration is incomplete.

Do not apply an import block or run `terraform import` without explicit authorization.

## Require authorization at the action boundary

Obtain explicit user authorization immediately before:

- any credentialed request to a live Ona organization;
- `terraform apply` or `terraform destroy`;
- applying import blocks or running `terraform import`;
- `terraform state rm`, `mv`, `replace-provider`, or another state mutation; and
- a provider upgrade that materially changes `.terraform.lock.hcl`.

Authorization for HCL editing or validation does not authorize a later live or state-changing command.

## Stop instead of guessing

Stop and explain when:

- Ona permissions or service-account-token capabilities are unclear;
- documentation and observed provider behavior disagree;
- a plan unexpectedly replaces, disables, or deletes an Ona object;
- a secret may enter configuration, logs, plan, state, generated files, or output;
- Query/import-generated configuration contains unsupported or unexplained fields; or
- import does not produce a no-op plan.

Never edit state files directly, apply generated configuration blindly, or broaden Ona permissions merely to make a provider error disappear.
