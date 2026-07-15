---
name: terraform-provider-development
description: Build, extend, review, test, or debug the Ona Terraform provider in Go with the Terraform Plugin Framework. Use for provider resources, data sources, ephemeral resources, schema/model changes, import behavior, state drift, generated docs/examples, local Terraform dev loops, acceptance tests, or questions like adding a resource, fixing a perpetual diff, or keeping tokens out of state.
---

# Terraform Provider Development

Build provider behavior that is correct under Terraform's plan/apply model, not just Go code that compiles. Most serious bugs are state-model bugs: perpetual diffs, duplicate creates, orphaned remote objects, secrets in state, and "provider produced inconsistent result after apply."

## Starting Decisions

- Use the Terraform Plugin Framework patterns already in this repository.
- Keep API wrapper code in `internal/client/**` separate from provider framework code in `internal/provider/**`.
- Treat `internal/api/go/**` as a copied/generated API subset. Do not hand-edit it for style-only changes.
- Keep docs and examples in sync with schemas by running `make generate` when relevant.

## Mental Model

1. Terraform operations move three documents: Config, Plan, and State. Use Plan in Create/Update, prior State in Read/Delete, and always write observed truth back to State.
2. Values can be known, null, or unknown. Do not collapse unknown into null or a Go zero value.
3. Each resource method handles one resource instance. Terraform Core already resolved graph references into known or unknown values.
4. Idempotency depends on truthful state. Persist remote IDs as soon as create succeeds, and make Read remove state only after a definitive not-found for the exact remote object.

## Workflow

Load the relevant reference before editing provider behavior. Use:

- `references/concepts-and-lifecycle.md` before changing lifecycle code or reasoning about Config, Plan, State, known, null, or unknown values.
- `references/core-implementation.md` when implementing provider, resource, data source, import, diagnostics, validators, or plan-modifier behavior.
- `references/advanced-primitives.md` when considering ephemeral resources, provider-defined functions, write-only arguments, resource identity, actions, or list resources.
- `references/secrets-and-sensitive-data.md` before adding or changing token, key, credential, or secret handling.
- `references/naming-conventions.md` when adding or renaming provider types, data sources, resource files, or model structs.
- `references/testing.md` before changing tests or acceptance-test coverage.
- `references/logging.md` before adding provider logs or diagnostics.
- `references/pitfalls.md` and `references/state-safety.md` before opening or reviewing a provider PR.

1. Model the API boundary first. Decide whether the behavior is a managed resource, data source, or ephemeral resource.
2. Add or update client wrapper behavior in `internal/client/**` when provider code needs a stable API abstraction.
3. Implement provider code in the existing package structure under `internal/provider/**`.
4. Register resources, data sources, and ephemeral resources in `internal/provider/provider.go`.
5. Align schema, model structs, Terraform field names, validators, plan modifiers, diagnostics, and import state behavior.
6. Add tests near the changed behavior.
7. Update examples and docs sources when users need new Terraform configuration.
8. Run generation and verification commands that match the change.

## Terraform Query

Terraform Query discovers existing managed objects through provider-defined
list resources. A query-enabled managed resource needs all of these pieces:

1. Implement `resource.ResourceWithIdentity` with the smallest immutable key
   that uniquely identifies the remote object. Set that identity after
   successful Create, Read, and Update operations.
2. Keep `resource.ResourceWithImportState`. Continue accepting the existing
   string import ID and also accept structured identity imports so
   Query-generated import blocks seed the ordinary state fields used by Read.
3. Implement and register a matching `list.ListResource`, and add its
   constructor to `OnaProvider.ListResources`. Provider configuration must set
   `ConfigureResponse.ListResourceData` so list resources receive the same
   authenticated client as managed resources.
4. Treat `list.ListRequest.IncludeResource` as a Terraform protocol request,
   not an Ona API field. Always return identity and display name. Populate
   `ListResult.Resource` from the same API-to-model mapping used by Read only
   when `IncludeResource` is true.
5. Respect `ListRequest.Limit`, paginate collection and parent lookups, emit
   deterministic results, and return API or mapping failures as diagnostics
   rather than incomplete success.
6. Never call token-issuing or secret-value endpoints during discovery. Keep
   write-only and unrecoverable values null in listed resource models, even if
   an API response includes them.
7. Add a `.tfquery.hcl` source example, generated list-resource docs, focused
   mapping/import unit tests, and a hermetic Terraform 1.14+ Query acceptance
   test that checks identity, display name, filters, limits, resource values,
   and secret omission where applicable.

Shared list helpers belong in `internal/provider/listutil`; API-specific
discovery stays in the resource package or an `internal/client` wrapper. Run
`make generate` after adding list schemas or examples.

## Golden Rules

- Persist ID fields immediately after a create API succeeds, before follow-up calls that can fail. See `references/state-safety.md` for the detailed rule and examples.
- `Read` must refresh every tracked attribute and remove state only when the remote API reports a definitive not-found for the exact object.
- After every framework `Get` or `Set`, append diagnostics and return on `HasError()`.
- Use `UseStateForUnknown()` on stable computed values that should not churn as "known after apply."
- Mark fields requiring recreation with replacement plan modifiers.
- Use sets for unordered remote collections so API ordering does not create diffs.
- Return planned known values consistently after apply unless the API intentionally canonicalizes them and the schema accounts for it.
- Use diagnostics for user-facing failures; do not panic.
- Mark sensitive attributes as sensitive, but use `references/secrets-and-sensitive-data.md` for the state decision tree and `references/logging.md` for log masking rules.
- Prefer ephemeral resources for issued temporary tokens that should not persist in state.

## Local Validation

- Install dependencies: `make install-dependencies`.
- Format: `make fmt`.
- Tests: `make test`.
- Build: `make build`.
- Lint: `make lint` when code changes warrant it.
- Generation: `make generate`, then `git diff --exit-code` when schemas, examples, docs, or codegen inputs change.

## Local Terraform Dev Loop

Use the README workflow when exercising the provider against Terraform:

```bash
mkdir -p .bin
go build -o .bin/terraform-provider-ona .
cat > terraformrc <<EOF
provider_installation {
  dev_overrides {
    "gitpod-io/ona" = "${PWD}/.bin"
    "ona-com/ona"  = "${PWD}/.bin"
  }
  direct {}
}
EOF
ONA_TOKEN="<api-token>" \
TF_CLI_CONFIG_FILE="${PWD}/terraformrc" \
terraform -chdir=dev/local-devloop plan -input=false
```

Do not commit `.bin/`, `terraformrc`, Terraform state, or real tokens.

## Done Criteria

A provider change is done when it has correct lifecycle behavior, tests for changed behavior, generated docs/examples when needed, no unintended generated diff, and a clear note about whether acceptance tests were run.

## When Stuck

- If Terraform shows a perpetual diff, compare schema flags, plan modifiers, API canonicalization, and collection ordering.
- If state is wrong after apply, inspect Create/Update return values and whether Read refreshes all tracked attributes.
- If a secret appears in output or state, use `references/secrets-and-sensitive-data.md` to decide whether it belongs in state, should become an ephemeral resource, or should be exposed only by reference.
- If generated docs are stale, change the schema/example source and rerun `make generate`.

## Reference Index

- `references/concepts-and-lifecycle.md` — the execution model and lifecycle rules.
- `references/core-implementation.md` — provider, resource, data source, schema, diagnostics, import, validators, and plan modifiers.
- `references/advanced-primitives.md` — newer framework/protocol features and when to use them.
- `references/secrets-and-sensitive-data.md` — canonical decision tree for sensitive inputs, secret outputs, and state exposure.
- `references/naming-conventions.md` — Terraform type names, Go type names, and file naming.
- `references/testing.md` — unit tests, acceptance tests, plan checks, import checks, and sweepers.
- `references/logging.md` — canonical provider logging, diagnostics, and log-masking guidance.
- `references/pitfalls.md` — common state-model failure modes.
- `references/state-safety.md` — canonical immediate-ID persistence rule and review findings from this provider translated into reusable checks.
