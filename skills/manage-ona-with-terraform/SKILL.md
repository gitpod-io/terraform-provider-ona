---
name: manage-ona-with-terraform
description: Safely create, edit, review, import, plan, and troubleshoot Terraform configurations using the gitpod-io/ona provider and ona_* resources. Use for Ona provider setup, resources, data sources, ephemeral resources, Terraform Query, imports, state safety, and provider diagnostics. Do not use for developing the provider's Go implementation.
---

# Manage Ona with Terraform

Manage Ona as an object graph, not as unrelated Terraform resources. Use this skill for consumer configuration; use versioned provider documentation for exact schemas.

## Understand the Ona provider model

- Authenticate to the Ona organization associated with `ONA_TOKEN`. Omit `host` for the default application host (`https://app.gitpod.io`); set `ONA_HOST` only for a non-default Ona application host.
- Prefer a personal access token for bootstrap and write workflows. Treat service-account tokens as suitable only for documented reads, automation starts, or explicitly confirmed write capabilities.
- Model runner capacity in dependency order: runner registration → runner environment class → project environment-class selection → optional project prebuilds → optional warm pool.
- Model organization access in dependency order: service account → group membership → organization role assignment. The token's organization is the boundary for organization-level role assignment.
- Treat SCM integrations, runner LLM integrations, Ona secrets, SSO client secrets, and runner custom-metrics credentials as secret-bearing resources with documented write-only values and explicit rotation markers.
- Treat runner tokens, service-account tokens, and webhook signing secrets as one-time or short-lived ephemeral outputs. Route them only to ephemeral-aware or write-only consumers.
- Use data sources for supported reads. Use Terraform Query list resources for discovery only; discovery does not create Terraform ownership.

These names describe the provider's domain model, not a complete resource catalog. Confirm that each type and behavior exists in the configured provider version.

## Resolve the exact provider contract

1. Inspect the consumer repository's Terraform files, `.terraform.lock.hcl`, `required_providers`, backend, and existing `ona_*` addresses.
2. Resolve the selected `gitpod-io/ona` version from the lock file, constraint, or installed provider. Preserve a compatible pin; never silently select, upgrade, or downgrade it.
3. Read `https://registry.terraform.io/providers/gitpod-io/ona/<version>/docs` for that exact version. Use `terraform providers schema -json` when the provider is installed.
4. Use [official Ona documentation](https://ona.com/docs/ona/getting-started) for product prerequisites, permissions, lifecycle, and credential capabilities.
5. Use official Terraform documentation only for Terraform mechanics such as [import blocks](https://developer.hashicorp.com/terraform/language/import) and [ephemeral/write-only values](https://developer.hashicorp.com/terraform/language/resources/ephemeral).

Do not emit runnable provider-specific HCL, import identities, lifecycle claims, or capability versions until the matching source has actually been consulted. If unavailable, use obvious placeholders and name the missing fact.

Treat provider names and relationships in this skill as navigation hints, not as proof of schema support. Do not use the repository that happens to contain this skill, remembered Terraform minimum versions, or an adjacent provider checkout as a substitute for the consumer's lock file and version-matched docs. When the consumer version is unknown, give a conceptual Ona workflow and request the version instead of claiming runnable HCL.

## Follow the Ona workflow

1. **Identify the Ona object.** Determine its organization, project, runner, service-account, or integration scope; whether it already exists; and whether Terraform should own it.
2. **Map prerequisites.** Follow Ona relationships rather than copying IDs: reference runners from environment classes, environment classes from projects and warm pools, projects from scoped resources, and groups/service accounts from access assignments.
3. **Choose the provider surface.** Use a managed resource for ownership, a data source for a supported read, an ephemeral resource for a non-persisted credential result, a list resource for Query discovery, or import for an existing supported object.
4. **Verify Ona-specific behavior.** Check required permissions, import identity, API deletion behavior, ForceNew/replacement fields, server defaults, censored read responses, and secret rotation markers in version-matched docs.
5. **Implement minimally.** Preserve repository conventions and unrelated edits. Use resource references instead of copied Ona UUIDs. Keep `provider "ona" {}` empty unless a non-default host is required; authenticate through `ONA_TOKEN`.
6. **Protect Ona credentials.** Never put tokens in HCL. Do not mistake `sensitive` for state exclusion. Use only documented write-only and ephemeral paths, and send issued credentials to an approved external secret target.
7. **Validate and interpret.** Format and validate locally. Run an authorized live plan, then explain Ona creates, updates, replacements, disables, deletions, credential rotations, and permission failures—not just Terraform action symbols.
8. **Import deliberately.** Use a declarative import block with the documented Ona identity form. Review generated HCL, especially omitted/censored secret fields and organization singleton settings. Require a no-op plan after import.
9. **Report ownership.** State which Ona objects Terraform will own, which it only reads, and what remote behavior occurs on replacement or destroy. Report formatting, initialization, validation, and planning results separately, including credentialed checks that were skipped.

Read [references/provider-workflows.md](references/provider-workflows.md) for Ona object graphs, authentication, Query, import identities, secret rotation, and diagnostic playbooks. Read [references/safety.md](references/safety.md) before credential, secret, import, state, replacement, or destroy work.

## Apply provider-specific guardrails

- Change both a write-only credential and its documented rotation marker; changing only the plaintext may not send an update.
- Do not expect import or refresh to recover write-only credentials censored by Ona.
- Do not use a service-account token to create automations; use a permitted user credential and verify executor ownership rules.
- Do not assume destroy always deletes. Some Ona resources disable, clear, restore prior settings, remove only Terraform state, retain history, or alter dependent automation triggers.
- Do not create a warm pool before its project has prebuilds enabled for the selected environment class.
- Do not treat Query-generated runner configuration as reviewed or imported configuration.

## Require explicit authorization

Obtain explicit authorization before any credentialed operation against a live Ona organization, `terraform apply`, `terraform destroy`, applying import blocks, `terraform import`, any `terraform state` mutation, or a provider upgrade that materially changes `.terraform.lock.hcl`.

Stop on an unexpected replacement/deletion, unclear Ona permission, possible secret persistence, documentation/provider disagreement, or non-no-op post-import plan.

## Reject these anti-patterns

- Printing or committing `ONA_TOKEN`, embedding tokens in provider blocks, or reading raw state for secrets.
- Guessing an `ona_*` type, argument, import identity, permission, or destroy behavior.
- Claiming an Ona provider or Terraform capability version without resolving it from the consumer configuration and current documentation.
- Copying the provider schema or maintaining a complete resource catalog in this skill.
- Using service-account tokens for writes without current official support.
- Running credentialed provider acceptance tests without explicit authorization.
- Blindly applying Query/import-generated configuration or treating validation as proof of safety.
- Editing state files directly or conflating state removal with remote Ona deletion.
