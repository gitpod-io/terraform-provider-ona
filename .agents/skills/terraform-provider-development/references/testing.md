# Testing

Testing is where robust providers are made. Most provider correctness is about the state model, and only acceptance tests with plan assertions catch state-model bugs.

## The core principle: test the state model, not the happy path

Provider bugs are almost never a malformed create call. They are state-model bugs: a perpetual diff, a duplicate create, a value that does not survive a round trip. Design tests to provoke those specifically. The single highest-value assertion in the whole suite is an empty plan immediately after apply (`plancheck.ExpectEmptyPlan()`); if it passes on a re-plan, you have ruled out the entire perpetual-diff family in one check. Put it on every managed resource where a no-op re-plan should be stable.

## The lifecycle a managed resource test should exercise

In order, within one `TestCase`. Each step targets a distinct failure mode from `pitfalls.md`:

1. **Create, then assert state** with `statecheck` (values landed correctly).
2. **Re-plan and assert empty** (no drift, no normalization diff).
3. **Import with `ImportStateVerify: true`** (proves `Read` rebuilds the whole resource from just an id/identity).
4. **Update in place, asserting the planned action** with `plancheck.ExpectResourceAction(..., ResourceActionUpdate)` (proves it updated rather than silently replacing or no-opping).
5. **Implicit destroy** at the end of the case (the framework tears down and verifies removal).

That sequence is the target shape for managed resources with create/read/update/import lifecycle behavior. Some provider changes are narrower: validators, client mapping, diagnostics, generated docs, data sources, and ephemeral resources still need targeted tests for the changed behavior, but not every item in this lifecycle sequence applies.

## Local dev override

Test against a locally built binary without publishing. Point a `.terraformrc` at your `GOBIN`:

```hcl
provider_installation {
  dev_overrides {
    "yourorg/foo" = "/home/you/go/bin"
  }
  direct {}
}
```

With a dev override, skip `terraform init` and iterate with `go install && terraform plan`.

## Two layers

- **Unit tests** for pure logic: client request building, response mapping, custom plan modifiers and validators. Fast, no infrastructure.
- **Acceptance tests** via the `terraform-plugin-testing` module, which actually run `terraform plan`/`apply` against the provider. In this repo, `make test-acc` sets `TF_ACC=1`; the backend can be a fake `httptest` server, a local stand-in, or a live Ona backend depending on the test and available configuration.

Division of labor: push as much correctness as possible down into unit tests, where you can exhaustively cover the fiddly edge cases (null vs unknown handling, malformed API responses, import id/identity parsing) fast and cheap. Reserve acceptance tests for what genuinely needs Terraform Core in the loop: planning behavior, import, and drift. Wire the in-process provider into acceptance tests with `ProtoV6ProviderFactories` so no separate build is needed.

## Canonical acceptance-test example

Do not maintain a second lifecycle-test code sample here. The canonical example lives in the Go test skill at [Terraform Provider Lifecycle Tests](../../go-tests/examples.md#terraform-provider-lifecycle-tests), and includes the immediate empty-plan assertion with `plancheck.ExpectEmptyPlan()`, import verification with `ImportStateVerify: true`, update coverage, and destroy verification. Use that example when adding or reviewing provider lifecycle tests, then adapt names and checks to the resource under test.

## Techniques that matter

- **Plan checks (`plancheck`)** on the planned action catch the most insidious bug class: a resource that plans an update or replace when nothing changed (a perpetual diff). The single most valuable assertion is "no changes" immediately after apply (`plancheck.ExpectEmptyPlan()`). Add it to lifecycle acceptance tests wherever the resource can re-plan without forced changes.
- **State checks (`statecheck`)** verify persisted state precisely.
- **Import verification** (`ImportStateVerify: true`) proves `Read` reconstructs the full resource from just an identifier, a strong correctness signal. Include an import step in every applicable resource test rather than in a separate isolated test, so each configuration variant is verified to import, not just one.
- **Test what the API normalizes.** If the backend lowercases a name, reorders a list, or trims whitespace, a naive test passes on create but the resource then shows a perpetual diff in real use. Write a step that deliberately sets a value the API will normalize, then assert an empty plan. This catches the most common real-world diff bug.
- **Sweepers** apply to live-backend acceptance tests that create real remote objects. Add them when a failed run can orphan infrastructure, leak cost, or wedge later runs because of unique names. Tests against fake `httptest` backends or disposable local stand-ins usually do not need sweepers.
- **Sensitive values.** Assert that secret attributes are marked sensitive and not echoed. Tests alone cannot catch a plaintext-in-state leak, so secret-output handling (per `secrets-and-sensitive-data.md`) is partly a review concern, but assert the sensitivity marking at least.

## What "done" looks like

A provider change is done when changed behavior has targeted tests, generated docs/examples are updated when needed, and the PR notes whether acceptance tests ran. For managed resources with create/read/update/import lifecycle behavior, the preferred acceptance coverage is: create and verify state, import with `ImportStateVerify`, update in place with a plan-action assertion, and show an empty plan on an immediate re-plan. If the re-plan is not empty, there is a state-model bug (usually a Read that does not refresh an attribute, an API that normalizes input, or a computed attribute missing `UseStateForUnknown`). See `pitfalls.md`.

## CI shape

- On every PR, the shared workflow runs `make fmt`, verifies no formatting diff, runs `make generate`, verifies no generated diff, then runs `make lint`, `make test`, and `make build`.
- `make test` runs `make test-unit test-acc`. `make test-acc` sets `TF_ACC=1` and runs the provider acceptance-test suite.
- Keep live-backend acceptance tests credential-gated. Fake `httptest` and disposable local-backend tests can run in PR CI; tests that create real Ona resources should run only when credentials/configuration are intentionally available and cleanup is defined.

## Hard-to-provision or fictional APIs

The framework does not mock Core, so do not try to fake Terraform. Mock at your **client** boundary for unit tests, and use a real or containerized backend for acceptance tests. If the backend is expensive or external, either stand up a disposable instance in CI (the HashiCups tutorial provider runs a local Docker container) or build a test double of the API. Fake your API, not Terraform.
