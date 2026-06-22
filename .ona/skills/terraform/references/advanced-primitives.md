# Advanced Primitives

Newer framework/protocol features beyond CRUD. Each maps to a Terraform version floor; verify the current floor against the framework changelog before relying on one, since these are the fastest-moving part of the ecosystem. Reach for them only where they clearly fit.

## Contents

- Provider-defined functions (Terraform 1.8+)
- Ephemeral resources (Terraform 1.10+)
- Write-only arguments (Terraform 1.11+)
- Resource identity (Terraform 1.12+)
- Actions (Terraform 1.14+)
- List resources (Terraform 1.14+)

## Provider-defined functions (Terraform 1.8+)

Pure functions callable in expressions as `provider::foo::name(...)`. Good for encoding/parsing helpers users would otherwise fake with `templatefile`. Implement a `Definition` (parameters, return, docs) and a `Run` method. No state, no side effects.

## Ephemeral resources (Terraform 1.10+)

A block (`ephemeral "type" "name" {}`) that **produces** a temporary value never written to plan or state. The right tool for short-lived credentials, tokens, or any value that must not persist. It has a real lifecycle exposed as RPCs you implement: `Open`, optionally `Renew`, and `Close`.

The lifecycle is the substantive difference from a data source: when a run outlives a server-enforced TTL, Terraform calls `Renew` to extend the lease (for example renewing a Vault lease mid-apply), then `Close`. Implement `ephemeral.EphemeralResource` (`Metadata`, `Schema`, `Open`), and optionally `Renew`, `Close`, `Configure`.

Ephemeral result values can only be referenced in **ephemeral contexts**: other ephemeral resources, ephemeral variables/outputs, provider config, and write-only arguments. Referencing one elsewhere is a plan-time error.

## Write-only arguments (Terraform 1.11+)

An **attribute** on a normal managed resource, flagged `WriteOnly: true`, that **consumes** a value, forwards it to the API, and is never stored in plan or state. The provider is the terminal point for the value. Use for secret inputs (passwords, tokens).

Key properties and constraints:

- Read the value from **config**, not plan or state. Write-only values are only available in the raw configuration; you cannot retrieve them like normal attributes.
- Accepts both ephemeral and non-ephemeral values (unlike other ephemeral constructs), though feeding it from an ephemeral resource is the recommended pattern.
- Prior state is always null, so a write-only argument **cannot produce a diff on its own**. Terraform sends it on every operation but nothing triggers an update when only the secret rotates. The convention is a paired `_version` attribute (which *is* stored in state): bump it to signal re-submission. Alternatively, store a hash of the value in private state to detect changes.
- `WriteOnly` attributes must also be `Required` or `Optional`. A nested write-only attribute requires all child attributes to be write-only too.
- Cannot be used with set attributes, set nested attributes, or set nested blocks.

Producer/consumer compose: an ephemeral resource produces a secret, a write-only argument consumes it, and neither persists it.

## Resource identity (Terraform 1.12+)

A separate, provider-defined data object stored in state alongside the resource, used to uniquely identify the remote object. Improves import (structured and typed instead of stringly-typed key parsing) and gives a stable anchor across schema versions.

Properties:

- The provider owns it entirely; the identity schema has no Required/Computed behaviors, the whole object is treated as Computed.
- Identity schemas hold only primitives (bool, string, number) and lists of primitives.
- It must correspond to at most one remote object per provider, let the provider determine existence and return state from it, and not change across the resource's lifecycle unless the schema is upgraded. The framework raises "Unexpected Identity Change" if it does.
- Each identity attribute is `RequiredForImport` or `OptionalForImport` (exactly one).
- Set identity data during Create, Read, and Update. Read must return it so import works.

Import by identity uses an `import` block with an `identity = { ... }` argument instead of a string ID; the user supplies either `id` or `identity`, never both. In the framework, wire it with `ImportStatePassthroughWithIdentity`. Adding identity support enables importing by default. The identity schema is versioned: bump the version and implement `ResourceWithUpgradeIdentity` for breaking changes, exactly like state upgraders.

Identity is the most mature of the post-CRUD primitives and is the foundation the list-resources feature depends on. Adopt it first.

## Actions (Terraform 1.14+)

An abstraction for provider-exposed **side-effects** against remote systems, for workflows that do not fit CRUD: disaster recovery, ad-hoc maintenance, invoking a Lambda, a cache invalidation, a playbook. Invoke directly from the CLI or as a trigger in a plan/apply workflow.

Current limitation: actions **cannot modify resource state** today (planned for the future). Until then they are for fire-and-observe side-effects only, which is the main reason to use them sparingly and only when they clearly fit.

## List resources (Terraform 1.14+)

An abstraction for **discovering unmanaged infrastructure**: searching for all objects of a resource type within a scope (every instance in an account, every network in a resource group). Used for bulk discovery and import.

- Authored as `list` blocks in a separate `.tfquery.hcl` file, driven by the `terraform query` command. This is a read-only discovery graph kept deliberately separate from the plan graph.
- `-generate-config-out` emits `import` plus `resource` blocks into a new file.
- Requires a corresponding resource implementation, because results rely on that resource's existing identity and schema. This is the concrete dependency on resource identity.
- On the provider side, `terraform query` triggers the `ListResource` RPC calling your `List` method; the request carries the list config and an `IncludeResource` flag (set by the `include_resource` argument or `-generate-config-out`), and the response streams results. Check `IncludeResource` early so you can return just identity data when full resource data is not needed, which matters for large scopes.
- SDKv2 has no native support; a compatibility path exists via mux and `RawV5Schemas`.

For agent-harness or reconciliation tooling, list resources are the interesting primitive: "discover everything of type X in this scope and emit import blocks" is a building block you can compose on.