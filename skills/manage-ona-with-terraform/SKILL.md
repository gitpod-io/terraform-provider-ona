---
name: manage-ona-with-terraform
description: Safely create, edit, review, import, plan, and troubleshoot Terraform configurations using the gitpod-io/ona provider and ona_* resources. Use for Ona provider setup, resources, data sources, ephemeral resources, Terraform Query, imports, state safety, and provider diagnostics. Do not use for developing the provider's Go implementation.
---

# Manage Ona with Terraform

Create minimal consumer Terraform for `gitpod-io/ona` while protecting credentials, state, and remote Ona objects. Treat schemas, import identities, lifecycle behavior, and permissions as version-dependent facts.

## Use canonical sources

Inspect the consumer repository before proposing changes:

- Read its Terraform files, `.terraform.lock.hcl`, provider constraints, and backend configuration.
- Inspect state metadata only when necessary; never read raw state to find secret values.
- Resolve the configured provider version from the lock file, constraints, or installed provider. Use its Registry documentation at `https://registry.terraform.io/providers/gitpod-io/ona/<version>/docs`; do not use `latest` for an existing pinned configuration.
- Inspect the installed schema with `terraform providers schema -json` when available.
- Consult official Ona documentation, starting at `https://ona.com/docs/ona/getting-started`, for credentials, permissions, lifecycle, and prerequisites.
- Consult official Terraform documentation for [import blocks](https://developer.hashicorp.com/terraform/language/import) and [ephemeral and write-only values](https://developer.hashicorp.com/terraform/language/resources/ephemeral).

Do not rely on remembered schemas or duplicate provider schemas in configuration guidance. When sources conflict, choose the more conservative supported behavior, stop before risky action, and report the discrepancy.

Do not emit runnable provider-specific resource HCL, exact import identities, capability claims, or version requirements until you have actually consulted the source for the selected version. If that source is unavailable, use unmistakable placeholders such as `ona_<documented_type>` and `<documented_argument>`, identify what must be verified, and stop short of claiming the configuration is valid. Never turn an unverified recollection into an example.

## Follow the workflow

1. **Understand the outcome.** Identify the remote Ona object or setting. Decide whether the task needs a managed resource, data source, ephemeral resource, list resource, or import. Identify every requested live state-changing operation.
2. **Inspect existing Terraform.** Preserve naming, modules, provider configuration, backend, and version conventions. Check existing resource addresses to avoid duplicates. Preserve unrelated user changes.
3. **Select an intentional version.** Use source `gitpod-io/ona`. Preserve a compatible existing constraint when appropriate. Never introduce an unpinned implicit `latest`, or silently downgrade or upgrade the provider. Explain any version change before making it.
4. **Verify behavior.** Check the matching Registry documentation or installed schema and the official Ona documentation. Record which versioned source was actually consulted. Never guess resource type names, argument names, import identities, Terraform or provider capability versions, replacement behavior, or permissions.
5. **Implement minimally.** Prefer small composable resources, explicit references, and variables for user-supplied identifiers and non-secret settings. Authenticate with `ONA_TOKEN`; use `ONA_HOST` only for a non-default Ona host. Never embed a token in Terraform.
6. **Protect secrets.** Explain that `sensitive = true` redacts presentation but does not prevent state storage. Prefer documented write-only arguments and ephemeral resources. Never print, log, commit, or expose secrets, and never inspect raw state for them. Store an issued credential only in an approved external secret target.
7. **Validate locally.** Run `terraform fmt`. Run `terraform init` when dependency installation is appropriate, then `terraform validate`. Run `terraform plan` only with explicit authorization for credentials and live access. Explain every proposed create, update, replacement, and deletion; validation alone does not prove an apply is safe.
8. **Import safely.** Prefer declarative HCL `import` blocks. Verify the exact identity format in documentation matching the configured provider. Review generated configuration before applying it. Require a no-op plan after import before declaring the migration complete.
9. **Report clearly.** List changed Terraform files. Report formatting, initialization, validation, and planning separately. State which credentialed checks were skipped. Highlight replacements, deletions, state mutations, and unresolved permission questions.

Read [references/provider-workflows.md](references/provider-workflows.md) for setup, resource selection, import, Terraform Query, secret rotation, or troubleshooting tasks. Read [references/safety.md](references/safety.md) before any credentialed, destructive, secret-handling, import, or state operation.

## Require authorization and stop safely

Obtain explicit user authorization immediately before:

- `terraform apply` or `terraform destroy`;
- applying import blocks or running `terraform import`;
- `terraform state rm`, `mv`, `replace-provider`, or any other state mutation;
- a provider upgrade that materially changes `.terraform.lock.hcl`; or
- any credentialed operation against a live Ona organization, including a live plan.

Stop and explain the impact when a plan unexpectedly replaces or deletes an object, removing configuration would delete a remote object, documentation disagrees with observed provider behavior, required permissions are unclear, a secret may enter state, or an import does not produce a no-op plan. Do not proceed until the risk is resolved and the required authorization is clear.

## Avoid anti-patterns

- Do not commit or print `ONA_TOKEN`, or place access tokens in provider blocks.
- Do not guess provider arguments, validation rules, import formats, permissions, or lifecycle behavior.
- Do not present remembered resource names, attributes, Terraform version thresholds, provider capabilities, or Registry availability as verified facts.
- Do not copy provider schemas or maintain a resource catalog in this skill or consumer configuration.
- Do not treat `terraform validate` as evidence that apply is safe.
- Do not blindly apply generated configuration.
- Do not edit Terraform state files directly.
- Do not run credentialed acceptance tests without explicit authorization.
- Do not assume service-account tokens can write unless current official documentation confirms the needed capability.
- Do not conflate removing an address from Terraform state with deleting its remote Ona object.
