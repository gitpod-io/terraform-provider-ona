# Secrets and Sensitive Data

How to handle secrets in a provider. The right tool depends on whether the secret flows *in* (an input) or *out* (a value the API generates and returns). Get this wrong and secrets land in plaintext state.

## The constraint that drives every decision

A managed resource's computed attributes live in state. There is **no write-only output**. If a resource exposes a generated secret as an attribute, that secret is written to state in plaintext.

`Sensitive: true` is **redaction only**: it hides the value in CLI output and the HCP Terraform UI and propagates sensitivity to references. It does **not** encrypt or remove the value from state. Always set it on secret fields, but never treat it as sufficient.

## Inputs: secrets flowing into a resource

Use a **write-only argument** (`advanced-primitives.md`): the value is sent to the API and never stored. Feed it from an ephemeral resource for an end-to-end path where neither producer nor consumer persists the secret. Pair with a `_version` attribute (or a private-state hash) to handle rotation, since a write-only argument cannot diff on its own.

## Outputs: secrets the API generates and returns

Decide by whether the secret can be re-obtained:

**1. If the token can be fetched or re-issued on demand → model it as an ephemeral resource, not a managed-resource attribute.** This is HashiCorp's explicit guidance: model a sensitive API object such as a token or secret as an ephemeral resource whenever possible. Because ephemeral results are never written to plan or state, the secret never lands there. Works when the value is retrievable or re-issuable at read time (for example an "issue token" operation that mints a fresh short-lived value each run). Does not work for a token shown exactly once at creation and never again.

**2. If the secret is durable and must be exposed as a managed attribute** (the user creates an API key once and needs to retrieve it), you cannot keep it out of state. Push protection down to the backend:
   - Mark the attribute `Sensitive: true` (redaction).
   - Accept it is in state and rely on an encrypted-at-rest remote backend plus tight access control. This is the supported answer now that remote backends and HCP Terraform encrypt state at rest.
   - Do **not** use the old PGP-key-in-state pattern; it is discouraged and being removed from HashiCorp-maintained providers.

**3. Best structural option where possible: expose only a reference, not the secret.** If the system can deposit the generated token into a secret store and return a handle (an ARN, a Vault path, a secret ID), make the durable managed attribute the handle and deliver the secret itself through an ephemeral resource that reads it back. The secret stays out of Terraform state entirely.

## The consumption nuance

A value can only pass between managed resources through state. If a downstream **managed** resource consumes the token as an ordinary argument, the token must be in state for Core to carry it there. The escape is the producer/consumer pairing: an ephemeral resource produces the token and the consumer accepts it as a write-only argument, so neither persists it. This only works if the consuming resource exposes a write-only argument; if it only has a normal attribute, the value is in state.

## Implementation details for the durable-attribute case

- **Never log the secret.** The framework does not redact your `tflog` output; one stray debug line defeats `Sensitive`.
- **Be careful in `Read` with write-once tokens.** If the API returns the token only at creation, do not overwrite the stored value with an empty/absent API response, or `Read` wipes it from state on the next refresh. Refresh the other attributes, leave the token as the prior state value.
- **Detect rotation without re-persisting the secret** by storing a hash in private state and pairing with a version-style trigger attribute.

## Quick decision summary

| Situation | Tool |
|---|---|
| Secret input to a resource | Write-only argument, ideally fed from an ephemeral resource |
| Secret output, re-fetchable or re-issuable | Ephemeral resource (never hits state) |
| Secret output, durable, must be exposed | `Sensitive: true` + encrypted backend + access control |
| Secret output, want it out of state entirely | Store in a secret manager, expose only a reference + ephemeral read-back |
| Display redaction only | `Sensitive: true` (necessary, never sufficient) |