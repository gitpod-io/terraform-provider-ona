# Safety boundaries

Read this reference before any credentialed, destructive, secret-handling, import, or state operation.

## Obtain explicit authorization

Require explicit user authorization immediately before:

- any credentialed operation against a live Ona organization, including `terraform plan` and authenticated Query;
- `terraform apply` or `terraform destroy`;
- applying declarative import blocks or running `terraform import`;
- `terraform state rm`, `terraform state mv`, `terraform state replace-provider`, or another state mutation; and
- a provider upgrade that materially changes `.terraform.lock.hcl`.

Authorization for inspection, editing, validation, or planning does not imply authorization for a later state-changing command. Explain the concrete target and expected impact before requesting authorization.

## Distinguish redaction from storage

Terraform `sensitive = true` suppresses ordinary display in CLI and UI output. It does not, by itself, prevent plaintext from being stored in plan or state. Backend encryption and access controls reduce exposure but do not turn an ordinary sensitive value into a non-persisted value.

Never claim that a sensitive variable, output, or attribute is absent from state without a documented write-only or ephemeral guarantee.

## Use write-only and ephemeral values correctly

- A documented **write-only argument** accepts a value without persisting that value in Terraform plan or state. Verify the exact provider and Terraform version behavior and any companion version or trigger argument.
- A documented **ephemeral value** is omitted from Terraform plan and state and may flow only through contexts Terraform permits for ephemeral values. Verify that every consumer in the chain supports it.
- These guarantees cover Terraform persistence boundaries; continue to follow provider, API, logging, shell, CI, and destination-secret handling requirements.
- Do not convert an ephemeral value into an ordinary local, output, resource argument, file, command-line argument, or log entry.

Stop if the supported path is incomplete or current documentation is ambiguous.

## Handle credentials conservatively

- Supply provider authentication through `ONA_TOKEN`; never embed it in a provider block, `.tfvars`, command line, committed environment file, or generated configuration.
- Use `ONA_HOST` only for a non-default Ona host.
- Never print, echo, log, commit, paste into diagnostics, or expose a token or issued secret.
- Never inspect raw state merely to locate or recover a secret.
- Store a credential that must be retained only in an approved external secret manager through a documented non-persisting path.
- Assume a service-account token is read-only or otherwise insufficient for writes unless current official Ona documentation explicitly supports the exact write operation and required permissions.
- Do not run credentialed provider acceptance tests without explicit authorization.

Redact diagnostics before sharing them, while preserving non-secret status codes, request context, and correlation information useful for troubleshooting.

## Separate remote deletion from state removal

Removing a managed resource block and applying normally asks Terraform to delete the remote Ona object. By contrast, `terraform state rm` stops Terraform from tracking an address without deleting the remote object at that moment. The object then exists outside that state and may conflict with future configuration or imports.

Never present these operations as interchangeable. Verify the intended ownership outcome, explain drift and re-import consequences, and obtain explicit authorization for either the remote deletion or the state mutation.

Do not edit a local or remote Terraform state file directly. Use supported Terraform state commands only after inspecting the backend, coordinating state locking, backing up according to repository policy, and obtaining authorization.

## Stop on unsafe plans or migrations

Stop and report before proceeding when:

- a plan contains an unexpected replacement or deletion;
- removing configuration would delete a remote Ona object;
- an import does not result in a no-op plan;
- generated configuration contains unexplained fields or secret-bearing values;
- provider diagnostics disagree with matching Registry documentation;
- required organization or object permissions are unclear; or
- any secret may enter configuration, logs, plan, state, generated files, or output.

For replacement, identify the force-new or identity-related change from current schema or diagnostics, explain downtime and credential consequences, and propose a non-destructive alternative only if documentation supports it. For deletion, identify the Terraform address and remote object without exposing private identifiers. Never apply merely to see what happens.
