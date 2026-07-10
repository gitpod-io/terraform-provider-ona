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
4. Idempotency depends on truthful state. Persist remote IDs as soon as create succeeds, and make Read accurately detect whether the object still exists.

## Workflow

1. Model the API boundary first. Decide whether the behavior is a managed resource, data source, or ephemeral resource.
2. Add or update client wrapper behavior in `internal/client/**` when provider code needs a stable API abstraction.
3. Implement provider code in the existing package structure under `internal/provider/**`.
4. Register resources, data sources, and ephemeral resources in `internal/provider/provider.go`.
5. Align schema, model structs, Terraform field names, validators, plan modifiers, diagnostics, and import state behavior.
6. Add tests near the changed behavior.
7. Update examples and docs sources when users need new Terraform configuration.
8. Run generation and verification commands that match the change.

## Golden Rules

- Persist ID fields immediately after a create API succeeds, before follow-up calls that can fail.
- `Read` must refresh every tracked attribute and remove state when the remote API reports a true not-found.
- After every framework `Get` or `Set`, append diagnostics and return on `HasError()`.
- Use `UseStateForUnknown()` on stable computed values that should not churn as "known after apply."
- Mark fields requiring recreation with replacement plan modifiers.
- Use sets for unordered remote collections so API ordering does not create diffs.
- Return planned known values consistently after apply unless the API intentionally canonicalizes them and the schema accounts for it.
- Use diagnostics for user-facing failures; do not panic.
- Mark sensitive attributes as sensitive, but remember that redaction does not keep values out of Terraform state.
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
- If a secret appears in output or state, revisit whether it should be an ephemeral resource rather than a sensitive attribute.
- If generated docs are stale, change the schema/example source and rerun `make generate`.
