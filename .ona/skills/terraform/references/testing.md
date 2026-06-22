# Testing

Testing is where robust providers are made. Most provider correctness is about the state model, and only acceptance tests with plan assertions catch state-model bugs.

## The core principle: test the state model, not the happy path

Provider bugs are almost never a malformed create call. They are state-model bugs: a perpetual diff, a duplicate create, a value that does not survive a round trip. Design tests to provoke those specifically. The single highest-value assertion in the whole suite is an empty plan immediately after apply (`plancheck.ExpectEmptyPlan()`); if it passes on a re-plan, you have ruled out the entire perpetual-diff family in one check. Put it on every resource.

## The lifecycle a resource test must exercise

In order, within one `TestCase`. Each step targets a distinct failure mode from `pitfalls.md`:

1. **Create, then assert state** with `statecheck` (values landed correctly).
2. **Re-plan and assert empty** (no drift, no normalization diff).
3. **Import with `ImportStateVerify: true`** (proves `Read` rebuilds the whole resource from just an id/identity).
4. **Update in place, asserting the planned action** with `plancheck.ExpectResourceAction(..., ResourceActionUpdate)` (proves it updated rather than silently replacing or no-opping).
5. **Implicit destroy** at the end of the case (the framework tears down and verifies removal).

That sequence is what "tested" means for a resource.

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
- **Acceptance tests** via the `terraform-plugin-testing` module, which actually run `terraform plan`/`apply` against the provider. They create real resources, so they are gated behind `TF_ACC=1` and skipped by default.

Division of labor: push as much correctness as possible down into unit tests, where you can exhaustively cover the fiddly edge cases (null vs unknown handling, malformed API responses, import id/identity parsing) fast and cheap. Reserve acceptance tests for what genuinely needs Terraform Core in the loop: planning behavior, import, and drift. Wire the in-process provider into acceptance tests with `ProtoV6ProviderFactories` so no separate build is needed.

## Shape of an acceptance test

```go
func TestAccWidgetResource(t *testing.T) {
    resource.Test(t, resource.TestCase{
        ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
        Steps: []resource.TestStep{
            { // create + read
                Config: testAccWidgetConfig("alpha", 3),
                ConfigStateChecks: []statecheck.StateCheck{
                    statecheck.ExpectKnownValue("foo_widget.test",
                        tfjsonpath.New("name"), knownvalue.StringExact("alpha")),
                },
            },
            { // import round-trips cleanly
                ResourceName:      "foo_widget.test",
                ImportState:       true,
                ImportStateVerify: true,
            },
            { // update in place, and assert the planned action
                Config: testAccWidgetConfig("beta", 5),
                ConfigPlanChecks: resource.ConfigPlanChecks{
                    PreApply: []plancheck.PlanCheck{
                        plancheck.ExpectResourceAction("foo_widget.test", plancheck.ResourceActionUpdate),
                    },
                },
            },
        },
    })
}
```

## Techniques that matter

- **Plan checks (`plancheck`)** on the planned action catch the most insidious bug class: a resource that plans an update or replace when nothing changed (a perpetual diff). The single most valuable assertion is "no changes" immediately after apply (`plancheck.ExpectEmptyPlan()`). Add it to every resource test.
- **State checks (`statecheck`)** verify persisted state precisely.
- **Import verification** (`ImportStateVerify: true`) proves `Read` reconstructs the full resource from just an identifier, a strong correctness signal. Include an import step in every applicable resource test rather than in a separate isolated test, so each configuration variant is verified to import, not just one.
- **Test what the API normalizes.** If the backend lowercases a name, reorders a list, or trims whitespace, a naive test passes on create but the resource then shows a perpetual diff in real use. Write a step that deliberately sets a value the API will normalize, then assert an empty plan. This catches the most common real-world diff bug.
- **Sweepers** clean up leaked test resources so a failed run does not orphan infrastructure or wedge later runs. Acceptance tests create real resources and tests panic, fail, or time out; without sweepers a flaky run leaks cost and can block later runs when a uniquely-named resource already exists. Write them alongside the tests, not after the first leak. Every mature provider has them.
- **Sensitive values.** Assert that secret attributes are marked sensitive and not echoed. Tests alone cannot catch a plaintext-in-state leak, so secret-output handling (per `secrets-and-sensitive-data.md`) is partly a review concern, but assert the sensitivity marking at least.

## What "done" looks like

A resource is not done until it has acceptance tests that: create and verify state, import with `ImportStateVerify`, update in place with a plan-action assertion, and show an empty plan on an immediate re-plan. If the re-plan is not empty, there is a state-model bug (usually a Read that does not refresh an attribute, an API that normalizes input, or a computed attribute missing `UseStateForUnknown`). See `pitfalls.md`.

## CI shape

- On every PR, fast and always: `go test` (unit) plus `golangci-lint`.
- On a schedule or on merge to main: acceptance tests (`TF_ACC=1`) against real backends or a local stand-in, with sweepers running before or after.

Keep acceptance tests out of the fast feedback loop deliberately; they are slow and create real infrastructure.

## Hard-to-provision or fictional APIs

The framework does not mock Core, so do not try to fake Terraform. Mock at your **client** boundary for unit tests, and use a real or containerized backend for acceptance tests. If the backend is expensive or external, either stand up a disposable instance in CI (the HashiCups tutorial provider runs a local Docker container) or build a test double of the API. Fake your API, not Terraform.