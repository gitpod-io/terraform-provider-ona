---
name: go-tests
description: >-
  Go test patterns using the Expectation pattern with cmp.Diff. MUST be consulted before writing ANY Go test code,
  including tests added as part of a larger feature implementation. Use when writing, adding, or fixing Go tests,
  when working with _test.go files, when implementing any Go feature that needs test coverage, when adding unit tests
  at the end of a task, or when verifying Go code with tests. Triggers on "Go test", "write tests", "add tests",
  "test coverage", "unit test", "_test.go", "verify with tests", "test the implementation", "add test cases",
  "table-driven test", "cmp.Diff", or any Go implementation task that includes a testing step.
---

# Go Tests

Table-driven tests using the Expectation pattern with `cmp.Diff`.

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

1. **Always use `t.Parallel()`** at test function and subtest level
2. **Never use `t.Parallel()` with `t.Setenv()`** - they panic together. If omitting `t.Parallel()`, add a `// not parallel: <reason>` comment explaining why.
3. **Single `cmp.Diff` assertion** comparing entire Expectation struct
4. **Capture errors as strings** in `Expectation.Err`, not with `t.Fatal`
5. **Use `t.Context()`** instead of `context.Background()`
6. **Use `t.Cleanup()`** for resource cleanup, not defer
7. **Use `t.Helper()`** in helper functions
8. **Never use testify** - use `cmp.Diff` only
9. **Never write timing-based tests** - see [No timing-based tests](#no-timing-based-tests)

## No timing-based tests

Tests that depend on real wall-clock time are flaky in CI under CPU and resource load. They pass locally and on a quiet runner, then fail on a busy one — eroding trust in CI and blocking main. Do not write them. PR reviewers will not approve them.

This means:

- No `time.Sleep` to wait for "something" to happen.
- No assertions that an operation took "at least" or "at most" some duration (`time.Since(start) > 100*time.Millisecond`).
- No `time.After` / `<-time.After(...)` as the success condition of a test.
- No tight `Eventually`-style polling loops with hardcoded short timeouts as the assertion (a 5ms poll interval and 50ms deadline will flake under load).
- No `runtime.Gosched()` or sleeps to "let the goroutine run".

Instead:

- **Inject a clock.** Take a `clock.Clock` (or equivalent) in the code under test and use a fake clock in the test. Advance it explicitly. This makes time-dependent behavior deterministic.
- **Synchronize on events, not durations.** Use channels, `sync.WaitGroup`, or callbacks the production code already exposes (or that you add for testability) to signal that a step has completed. Block on those, not on `time.Sleep`.
- **Make concurrency observable.** If a test needs to know that a background worker reached a state, expose a hook (a channel, a status method) and wait on it with a generous deadline (`t.Context()` or several seconds) — the deadline is a safety net, not the assertion.
- **Trigger events from inside the SUT's call path, not from a parallel timed goroutine.** If event X must happen after the system reaches state Y, fire X from inside the mock or callback the SUT invokes when it reaches Y. Example: to test cancellation mid-retry, call `cancel()` from inside the failing operation itself, not from a `go func() { time.Sleep(…); cancel() }()`.
- **Don't probe process-global state to infer what your code did.** `runtime.NumGoroutine()`, process-wide counters, and heap size are polluted by other parallel tests. Have the component expose a signal it owns (e.g. a `done <-chan struct{}` returned from `startWorker`) and assert on that.
- **Test the contract, not the schedule.** If you find yourself wanting to assert "this ran after 30s", you are testing the scheduler. Test that the scheduled function does the right thing when invoked, and test the scheduler separately with a fake clock.

If you genuinely cannot avoid waiting on real time (e.g. an integration test against an external system), use a generous deadline and document why in a comment above the wait. Never use a tight bound.

## cmp.Diff Options

```go
// Ignore non-deterministic fields
cmpopts.IgnoreFields(Response{}, "CreatedAt", "UpdatedAt")

// Handle unexported fields (generated code)
cmpopts.IgnoreUnexported(db.Environment{})

// Protobuf messages
protocmp.Transform()

// Combine options
cmp.Diff(tc.Expected, got,
	protocmp.Transform(),
	cmpopts.IgnoreFields(db.Prebuild{}, "CreateTime", "Edges"),
	cmpopts.EquateEmpty(),
)
```

## Common Patterns

For HTTP handler tests, database tests with ExistingObjects, and Setup functions, see [examples.md](examples.md).

## Anti-Patterns

- `t.Fatal` for action errors → use `Expectation.Err`
- `len(results) != expected` → compare full results
- `context.Background()` → use `t.Context()`
- `testify/assert` → use `cmp.Diff`
- Missing `t.Parallel()` on independent tests
- `time.Sleep`, real-time deadlines, or duration-based assertions → inject a clock and synchronize on events (see [No timing-based tests](#no-timing-based-tests))
- `runtime.NumGoroutine()` or other process-global state as an assertion → expose a signal from the component
- Timed goroutine that calls `cancel()` / injects an error mid-test → trigger it from inside the mock the SUT is calling
