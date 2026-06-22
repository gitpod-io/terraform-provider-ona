---
name: terraform-provider-development
description: Write high-quality Terraform providers in Go with the Terraform Plugin Framework. Use this skill whenever the task involves building, extending, reviewing, or debugging a Terraform provider, a custom resource or data source, an ephemeral resource, a provider-defined function, resource identity, a write-only argument, list resources, or provider actions, even when the user does not say the word "provider" explicitly (for example "add a resource to our terraform plugin", "why does my resource show a perpetual diff", "how do I keep this token out of state", "write a CRUD resource for our API"). It encodes the framework's execution model, the schema and state machinery, naming conventions, testing, secret handling, and the pitfalls that separate a robust provider from one that corrupts state or leaks secrets.
---

# Terraform Provider Development

Build providers that are correct under Terraform's plan/apply model, not just code that compiles. Most provider bugs are not syntax errors; they are state-model errors (perpetual diffs, duplicate creates, secrets in state, "inconsistent result after apply"). This skill front-loads the model so the implementation follows from it.

## Non-negotiable starting decisions

- **Use the Terraform Plugin Framework, never SDKv2, for new code.** SDKv2 is feature-frozen and lacks ephemeral resources, write-only arguments, functions, and identity. Only touch SDKv2 when maintaining an existing SDKv2 codebase, and migrate incrementally with `terraform-plugin-mux`.
- **Start from the scaffolding**, never a blank directory: `hashicorp/terraform-provider-scaffolding-framework`. It wires the plugin server, Makefile, lint, CI, GoReleaser, and docs generation that everything else assumes.
- **Repo name must be `terraform-provider-<NAME>`.** The registry and `terraform init` depend on it.

## The mental model (load this before writing any code)

Read `references/concepts-and-lifecycle.md` for the full version. The compressed form:

1. **Three documents flow through every operation: Config, Plan, State.** Config is what the user wrote. Plan is the proposed next state. State is the last observed truth. You read from the right one per method (Plan in Create/Update, prior State in Read/Delete) and always write the result back to State.
2. **Every value is in one of three conditions: known, null, or unknown.** Unknown means "known after apply." Treating unknown as null or as a zero value is the root of a large fraction of provider bugs. Never collapse the three.
3. **Each resource method sees only one resource instance, pre-resolved.** The provider is a gRPC subprocess; Core sends just that instance's slice and has already resolved cross-resource references into concrete or unknown values. You never traverse the graph yourself.
4. **Idempotency is Core's job, enforced through state, not yours.** Core calls Create only for an address absent from state. Your duty is to keep state truthful: persist the ID the instant the remote object exists, and make Read detect existence honestly. Everything past the crash window is the remote API's uniqueness/idempotency-token problem.

## Workflow

Follow this order. Each step has a dedicated reference.

1. **Model the target API first.** Write or wrap a thin Go client. Decide what is a managed resource (full lifecycle), a data source (read-only lookup), or an ephemeral resource (temporary, never stored). See `references/concepts-and-lifecycle.md` and `references/advanced-primitives.md`.
2. **Implement the provider, resources, and data sources.** Provider builds the client once in `Configure` and threads it through. Resources implement Create/Read/Update/Delete plus Import. See `references/core-implementation.md`.
3. **Name everything consistently.** Singular vs plural type names, the `Collection` convention for plural data sources, file and struct naming. See `references/naming-conventions.md`.
4. **Handle secrets deliberately.** Decide per secret: ephemeral resource (preferred for issued/fetchable tokens), write-only argument (for secret inputs), or Sensitive-plus-encrypted-backend (for durable secret outputs). See `references/secrets-and-sensitive-data.md`.
5. **Adopt newer primitives only where they clearly fit.** Functions, resource identity, ephemeral resources, actions, list resources. Mind the Terraform version floors. See `references/advanced-primitives.md`.
6. **Test against the state model.** Acceptance tests with plan checks that assert "no changes", import round-trips, and sweepers. See `references/testing.md`.
7. **Document and publish.** Generate docs from schemas and examples with `tfplugindocs`; handle release and registry publishing as a separate deliberate workflow.
8. **Review against the pitfalls list** before considering any resource done. See `references/pitfalls.md`.

## Golden rules (the checklist that prevents the worst bugs)

Apply these to every resource. Each maps to a concrete failure mode explained in the references.

- **Persist the ID immediately after the create API succeeds**, before any further step that could fail. Otherwise a mid-create failure orphans the remote object and the next apply creates a duplicate.
- **`Read` refreshes every tracked attribute and calls `RemoveResource` on a 404.** A lazy Read breaks drift detection and Core's create/update gating.
- **Guard after every `Get`/`Set`**: `resp.Diagnostics.Append(...)` then `if resp.Diagnostics.HasError() { return }`. `Append` does not stop execution; proceeding with a failed decode uses garbage values. Check `HasError()`, not `len(...)`, so warnings do not halt.
- **Put `UseStateForUnknown()` on computed attributes that are stable across updates.** Without it, every plan shows "known after apply" and churns noise.
- **Mark anything that cannot be changed in place `RequiresReplace`.** Otherwise updates silently no-op.
- **`Sensitive: true` is redaction only; it does not keep a value out of state.** For real secrets, use the decision tree in the secrets reference. Never log a secret with `tflog`; the framework does not redact your logs.
- **Log with `tflog`, never `slog`/`log`/stdout.** A provider is a subprocess; logs must travel back through the plugin channel, and direct stdout writes corrupt or lose them. Mask secret fields and keep user-facing messages in diagnostics, not logs. See `references/logging.md`.
- **Use `Set` (not `List`) when order is not meaningful**, or API reordering shows up as a diff.
- **Return exactly what was planned for known values**, or Core raises "Provider produced inconsistent result after apply."
- **Reserve `List`/`ListResource` naming for the actual List Resources primitive.** A plural data source is named with the `Collection` convention, not `List`.

## Quality bar

A resource is done when: it creates, reads, updates, destroys, and imports cleanly; an immediate re-plan shows no changes (assert this with a plan check); secrets are handled per the decision tree; and it has acceptance tests including an import verify step.

## Reference index

- `references/concepts-and-lifecycle.md` — the three-document model, known/null/unknown, the apply lifecycle, per-instance scoping, and how idempotency works. Read first.
- `references/core-implementation.md` — provider, resource CRUD, data sources, schema, framework types, diagnostics, plan modifiers, validators, import. The main implementation reference.
- `references/advanced-primitives.md` — ephemeral resources, provider-defined functions, write-only arguments, resource identity, actions, list resources, with version floors.
- `references/secrets-and-sensitive-data.md` — the decision tree for sensitive inputs and secret outputs.
- `references/naming-conventions.md` — type names, the singular/Collection convention, struct and file naming, get vs list data sources.
- `references/testing.md` — unit and acceptance testing, plan/state checks, import verification, sweepers, dev overrides.
- `references/logging.md` — why `tflog` not `slog`, the API and levels, secret masking, and logs vs diagnostics.
- `references/pitfalls.md` — the catalog of state-model failure modes and how to avoid each.
