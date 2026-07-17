# Provider workflows

Use these workflows as decision guides. Obtain names, attributes, identities, validation rules, and lifecycle details from documentation matching the configured `gitpod-io/ona` provider version or from the installed schema.

Until those sources have actually been consulted, provide only non-runnable placeholder HCL. Do not infer an Ona resource type from the product object's name, invent representative arguments, state a minimum Terraform version from memory, or claim that a Registry version exists or is absent. Say which fact remains unverified.

## Set up a new provider intentionally

1. Inspect the Terraform CLI version, module structure, backend, existing provider conventions, and organizational version policy.
2. Select an explicit provider constraint based on compatibility requirements and an intentionally chosen released version. Do not create an implicit `latest` dependency.
3. Declare `source = "gitpod-io/ona"` and the chosen constraint in `required_providers` using the repository's existing style. Do not add a numeric version or `required_version` example until its source has been checked.
4. Keep authentication outside HCL. Supply `ONA_TOKEN` through the approved execution environment. Set `ONA_HOST` only for a non-default Ona host.
5. Run `terraform init` only when installing dependencies is appropriate. Review the resulting lock-file changes and explain the selected version and checksums.

Use placeholders such as `<provider-version>`, `<resource-address>`, and `<remote-identity>` in guidance. Never insert a real token or private identifier.

## Resolve matching Registry documentation

1. Prefer the exact selected version in `.terraform.lock.hcl`.
2. If no lock entry exists, evaluate the configured constraint and the version actually installed by `terraform init` or reported by Terraform.
3. Open `https://registry.terraform.io/providers/gitpod-io/ona/<version>/docs`, replacing `<version>` with that exact version.
4. Cross-check a machine-readable installed schema with `terraform providers schema -json` when available.
5. If code, lock file, installed schema, and Registry documentation disagree, do not improvise. Describe the mismatch and use the most conservative supported behavior.

## Select the correct Terraform abstraction

- Choose a **managed resource** when Terraform should own the remote object's lifecycle and changes.
- Choose a **data source** when configuration should read an existing object without owning it.
- Choose an **ephemeral resource** when a temporary value or credential must be obtained without persisting it in plan or state and the documented consumer accepts ephemeral values.
- Choose a **list resource** for Terraform Query discovery when the installed Terraform CLI and selected provider version both support the documented list capability.
- Choose **import** when a supported existing remote object should become managed at a stable Terraform address.

Verify the abstraction exists in the selected provider version. Do not substitute a similarly named type based on memory.

## Import declaratively

1. Read [safety.md](safety.md), inspect the backend, and confirm that no other address already manages the object.
2. Verify the exact import ID or identity format in the matching Registry documentation.
3. Add a declarative block with placeholders until the user supplies the reviewed identity:

   ```hcl
   import {
     to = ona_<type>.<name>
     id = var.<import_identity>
   }
   ```

   Use the documented identity form; do not assume it is a single string if the current provider supports another form.
4. If Terraform generates configuration, inspect every argument, secret-bearing field, default, and lifecycle consequence. Never apply generated configuration blindly.
5. Show the proposed import and obtain explicit authorization before applying the import block or running any imperative import command.
6. After the state operation, run an authorized plan. Require a no-op result. If Terraform proposes any change, replacement, or deletion, stop and reconcile configuration, identity, version, and provider behavior.

## Use Terraform Query

1. Confirm with `terraform version` and current official Terraform documentation that the installed CLI supports the Query workflow; follow the documentation for that CLI version. Do not state a minimum version from memory.
2. Confirm the selected `gitpod-io/ona` provider version exposes the needed list resource in its Registry documentation or installed schema.
3. Keep query files and filters minimal, use placeholders for private identifiers, and avoid selecting or rendering secret material.
4. Initialize dependencies only when appropriate, then run the documented query command. Treat any authenticated query against an Ona organization as a credentialed live operation requiring explicit authorization.
5. Explain that Query discovers or reports objects; it does not automatically establish managed-resource ownership. Use a reviewed import workflow when management is desired.

Do not hard-code a provider or Terraform version in reusable guidance. State the capabilities required, resolve installed versions, and report an unsupported-version result instead of silently upgrading.

## Rotate a secret without persisting plaintext

1. Read [safety.md](safety.md) and verify the selected provider and Terraform versions document the required write-only or ephemeral path end to end. Do not name a concrete Ona resource, write-only argument, companion version argument, or minimum version until the matching sources confirm it.
2. Trace the secret from issuance to consumption. Every intermediate expression, variable, output, resource argument, and destination must support the required non-persistence guarantee.
3. Use a documented write-only argument for a managed resource or a documented ephemeral resource as appropriate. Do not infer support from an attribute merely being marked sensitive.
4. Send a credential that must be retained directly to an approved external secret manager through a documented non-persisting path. Never print it or recover it from raw state.
5. Present the plan consequences without secret values and obtain authorization before any credentialed or state-changing operation.
6. Verify rotation through non-secret metadata or behavior documented by Ona. Revoke the old credential only with explicit authorization and a recovery plan.

If any step could persist plaintext, stop and explain the unsafe boundary.

## Troubleshoot authentication and permissions

1. Classify the failure: missing credentials, invalid or expired credentials, wrong host, insufficient organization or object permissions, unsupported token capability, or network/TLS failure.
2. Confirm only whether `ONA_TOKEN` is set in the intended process; never echo or log its value. Confirm `ONA_HOST` only when a non-default host is expected.
3. Check official Ona documentation for the current credential type, required role or permission, scope, lifecycle, and prerequisites.
4. Compare the failing operation with provider-version documentation and diagnostics. Preserve redaction when sharing errors.
5. Do not respond to a permission failure by broadening access, minting credentials, or assuming service-account write capability. Propose the least privilege documented for the exact operation and ask an authorized administrator when needed.

## Use validation commands proportionately

- Run `terraform fmt` after HCL edits; report files changed by formatting.
- Run `terraform init` when provider/module installation or lock resolution is appropriate. Do not materially change a lock file through upgrade behavior without authorization.
- Run `terraform validate` after initialization when required. Treat it as structural validation, not a safety verdict.
- Run `terraform plan` only after explicit authorization for credentials and live Ona access. Summarize creates, in-place updates, replacements, deletions, imports, and unknown values separately.
- Never run `apply`, `destroy`, import application, or state mutation merely because formatting, validation, or planning succeeded.
