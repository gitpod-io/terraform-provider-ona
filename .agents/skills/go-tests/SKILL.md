---
name: go-tests
description: Go test patterns for this Terraform provider using table-driven tests, the Expectation pattern, and cmp.Diff. MUST be consulted before writing Go test code, including tests added as part of a larger feature implementation. Use when writing, adding, or fixing Go tests; working with _test.go files; implementing Go behavior that needs test coverage; adding unit tests; testing Terraform provider mapping/model/diagnostic behavior; adding acceptance tests with Terraform Plugin Testing; or verifying Go code with tests. Triggers on "Go test", "write tests", "add tests", "test coverage", "unit test", "_test.go", "verify with tests", "test the implementation", "add test cases", "table-driven test", "cmp.Diff", or any Go implementation task that includes a testing step.
---

# Go Tests

Table-driven tests using the Expectation pattern with `cmp.Diff`.

For Terraform provider lifecycle behavior, also use the `terraform-provider-development` skill. Provider tests must prove Terraform state, plan, import, diagnostics, and drift behavior, not only Go return values.

## Required Pattern

```go
func TestFunctionName(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result ResultType
		Err    string
	}

	tests := []struct {
		Name     string
		Input    InputType
		Expected Expectation
	}{
		{
			Name:  "descriptive_snake_case_name",
			Input: inputValue,
			Expected: Expectation{
				Result: expectedValue,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			result, err := FunctionUnderTest(tc.Input)
			if err != nil {
				got.Err = err.Error()
			} else {
				got.Result = result
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("FunctionUnderTest() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
```

## Rules

1. **Use `t.Parallel()` by default** at test function and subtest level when the test is isolated.
2. **Never use `t.Parallel()` with `t.Setenv()`** - they panic together. If omitting `t.Parallel()`, add a `// not parallel: <reason>` comment explaining why.
3. **Use one `cmp.Diff` assertion** comparing the whole `Expectation` struct for pure Go unit tests.
4. **Capture action errors as strings** in `Expectation.Err`. Use `t.Fatal` or `t.Fatalf` only for setup failures where the test cannot continue.
5. **Use `t.Context()`** instead of `context.Background()` unless the code under test specifically needs a derived context constructor.
6. **Use `t.Cleanup()`** for resource cleanup, not defer.
7. **Use `t.Helper()`** in helper functions.
8. **Never use testify** - use `cmp.Diff` and Terraform Plugin Testing checks.
9. **Never write timing-based tests** - see [No timing-based tests](#no-timing-based-tests).
10. **Use Terraform Plugin Testing checks for acceptance tests**. `resource.Test`, `resource.TestCheckResourceAttr`, import checks, plan checks, and destroy checks are the right assertions for provider lifecycle tests.
11. **Use diagnostics summaries or details in expectations** when testing Terraform Plugin Framework diagnostics.
12. **Do not hand-edit `internal/api/go/**` for style-only test changes**. That directory is a copied/generated API subset.

## No timing-based tests

Tests that depend on real wall-clock time are flaky in CI under CPU and resource load. They pass locally and on a quiet runner, then fail on a busy one - eroding trust in CI and blocking main. Do not write them. PR reviewers will not approve them.

This means:

- No `time.Sleep` to wait for "something" to happen.
- No assertions that an operation took "at least" or "at most" some duration (`time.Since(start) > 100*time.Millisecond`).
- No `time.After` / `<-time.After(...)` as the success condition of a test.
- No tight `Eventually`-style polling loops with hardcoded short timeouts as the assertion (a 5ms poll interval and 50ms deadline will flake under load).
- No `runtime.Gosched()` or sleeps to "let the goroutine run".

Instead:

- **Inject a clock.** Take a `clock.Clock` (or equivalent) in the code under test and use a fake clock in the test. Advance it explicitly. This makes time-dependent behavior deterministic.
- **Synchronize on events, not durations.** Use channels, `sync.WaitGroup`, or callbacks the production code already exposes (or that you add for testability) to signal that a step has completed. Block on those, not on `time.Sleep`.
- **Make concurrency observable.** If a test needs to know that a background worker reached a state, expose a hook (a channel, a status method) and wait on it with a generous deadline (`t.Context()` or several seconds) - the deadline is a safety net, not the assertion.
- **Trigger events from inside the SUT's call path, not from a parallel timed goroutine.** If event X must happen after the system reaches state Y, fire X from inside the mock or callback the SUT invokes when it reaches Y. Example: to test cancellation mid-retry, call `cancel()` from inside the failing operation itself, not from a `go func() { time.Sleep(...); cancel() }()`.
- **Don't probe process-global state to infer what your code did.** `runtime.NumGoroutine()`, process-wide counters, and heap size are polluted by other parallel tests. Have the component expose a signal it owns (e.g. a `done <-chan struct{}` returned from `startWorker`) and assert on that.
- **Test the contract, not the schedule.** If you find yourself wanting to assert "this ran after 30s", you are testing the scheduler. Test that the scheduled function does the right thing when invoked, and test the scheduler separately with a fake clock.

If you genuinely cannot avoid waiting on real time (e.g. an integration test against an external system), use a generous deadline and document why in a comment above the wait. Never use a tight bound.

## cmp.Diff Options

```go
cmpopts.IgnoreFields(Response{}, "CreatedAt", "UpdatedAt")

cmpopts.IgnoreUnexported(SomeType{})

protocmp.Transform()

cmpopts.EquateEmpty()

cmp.Diff(tc.Expected, got,
	protocmp.Transform(),
	cmpopts.IgnoreFields(v1.Project{}, "CreatedAt", "UpdatedAt"),
	cmpopts.EquateEmpty(),
)
```

## Common Patterns

For HTTP handler tests, Setup functions, protobuf tests, Terraform provider lifecycle tests, model-to-request mapping tests, diagnostics tests, and helper functions, see [examples.md](examples.md).

## Anti-Patterns

- `t.Fatal` for action errors -> use `Expectation.Err`
- `len(results) != expected` -> compare full results
- `context.Background()` -> use `t.Context()`
- `testify/assert` -> use `cmp.Diff`
- Missing `t.Parallel()` on independent tests
- `time.Sleep`, real-time deadlines, or duration-based assertions -> inject a clock and synchronize on events (see [No timing-based tests](#no-timing-based-tests))
- `runtime.NumGoroutine()` or other process-global state as an assertion -> expose a signal from the component
- Timed goroutine that calls `cancel()` / injects an error mid-test -> trigger it from inside the mock the SUT is calling
- Mocks that only ever succeed -> test the failure direction (see [Coverage Definition of Done](#coverage-definition-of-done))
- Terraform acceptance tests that only create -> also cover update, empty plan, import, delete/read-not-found, or diagnostics when relevant
- Tests that collapse unknown/null Terraform values to Go zero values

## Coverage Definition of Done

Missing or weak tests are the most common PR-review finding in this repo. Run this checklist before considering test work complete.

1. **Every new branch gets a table case - especially error returns and `enabled=false` gates.**

   - Each new `if err != nil`, nil guard, unknown/null guard, or disabled/false path needs a case that drives it.
   - A config block present with `Enabled: false` is a different branch from a missing block - cover both.
   - Verify each case actually reaches the new branch: a case that trips an earlier guard does not cover the branch below it.

2. **Every fake or mock that exposes an error field has at least one case setting it.**

   - A fake with `return f.result, f.err` where no test ever sets `f.err` means the error-propagation path is untested.
   - If the field is genuinely unreachable, remove it instead.

3. **Every behavior claim in the PR description maps to a named test case.**

   Before opening the PR, list the claims and pair each with the case that proves it:

   ```text
   "removes state when the project is not found" -> TestAccProjectResourceReadRemovesNotFound
   "unknown min size is rejected before apply"   -> TestCreateWarmPoolRequest/rejects_unknown_min_size_before_apply
   ```

   A claim with no case is either untested or not a real behavior of the change. Fix PRs especially: the scenario the fix addresses needs a regression case that fails without the fix.

4. **Anti-vacuity check: prove the test can fail.**

   - Temporarily invert or revert the code change and confirm the new case fails, then restore.
   - Or assert on a fake's call count to prove the path under test actually executes (e.g. `Calls: 1` in the `Expectation`).
   - A test that passes both with and without the change verifies nothing.

5. **Mocks that only ever succeed are a smell.**

   - For every mocked dependency whose error the code handles, test the failure direction.
   - Assert what the caller does when the dependency errors: early return, diagnostic summary, state removal, skipped update - not just that an error string comes back.

6. **Provider lifecycle changes need Terraform-shaped coverage.**

   - Resource tests should cover create, update, empty plan, import, and destroy/read-not-found when the behavior supports it.
   - Data source tests should cover successful reads and useful diagnostics.
   - Ephemeral resource tests should prove secrets or temporary tokens do not persist in Terraform state.
