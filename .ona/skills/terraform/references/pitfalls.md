# Pitfalls

The catalog of state-model failure modes. Review every resource against this list before considering it done. Each entry is a real bug class, not a style nit.

## Perpetual diffs

The plan shows a change on every run even when nothing changed. Causes:

- `Read` does not refresh an attribute, so state drifts from reality.
- The API normalizes what you sent (lowercases, reorders, trims), so the stored value never matches config.
- A computed attribute lacks `UseStateForUnknown()`, so it shows "known after apply" every plan.

Catch it with a `plancheck.ExpectEmptyPlan()` assertion immediately after apply. Fix by refreshing all attributes in Read, normalizing config to match the API's canonical form, or adding the plan modifier. For the normalization case specifically, add a test step that sets a value the API will rewrite and assert an empty plan (see `testing.md`).

## "Provider produced inconsistent result after apply"

Core enforces that the final state matches the plan wherever the plan was known. If you set an attribute to a value different from a known planned value, you get this error. Either mark the attribute computed/unknown in the plan, or return exactly what was planned for known values.

## Mishandling unknown

During plan, referenced values can be unknown. Do not treat unknown as null or as an empty string. Computed-only outputs should generally be unknown at plan time unless you can legitimately carry the prior state value forward with `UseStateForUnknown()`. Collapsing the known/null/unknown trichotomy is the underlying cause of several bugs on this list.

## Duplicate creates

The create API succeeds, a later step in `Create` fails, and you return without writing state. The remote object now exists but no address points to it, so the next apply creates a second one. Fix: write the ID into state the instant the create API returns success, before any step that could fail. Beyond the in-process case, rely on the remote API's uniqueness constraints or idempotency tokens for crash-window safety.

## Lazy Read breaking drift detection

`Read` must refresh every tracked attribute and call `resp.State.RemoveResource(ctx)` on a 404. If Read does not remove a deleted resource, Core never plans a recreate. If it does not refresh an attribute, drift goes undetected and you also risk a perpetual diff.

## Forgetting RequiresReplace

If an attribute cannot be changed in place by the API but is not marked `RequiresReplace`, updates silently no-op: the plan looks fine, the apply does nothing, and reality never matches config. Mark every immutable attribute.

## Set vs List

Using `List` when order is not meaningful makes harmless API reorderings show up as diffs. Use `Set` for unordered collections.

## Secret leakage

- `Sensitive: true` is redaction only; it does not keep a value out of state. See `secrets-and-sensitive-data.md`.
- The framework does not redact your `tflog` output. A stray debug log of a secret defeats `Sensitive` entirely. Never log secret values; mask secret fields on the context (see `logging.md`).
- Returning a generated secret as a managed attribute persists it to state in plaintext. Use the secrets decision tree instead.

## Write-once token wiped on refresh

If the API returns a token only at creation, a Read that overwrites the stored value with the empty/absent API response erases it from state on the next refresh. Refresh the other attributes and leave the token as the prior state value.

## Partial-failure orphans

A multi-step create that fails halfway and returns without persisting what was created leaves orphaned remote objects. Write whatever state was actually created before returning the error, so the next apply reconciles instead of duplicating.

## Skipping the guard after Get/Set

`resp.Diagnostics.Append(...)` does not stop execution. Without the immediately following `if resp.Diagnostics.HasError() { return }`, you proceed with a partially-decoded struct and make API calls with garbage values. Check `HasError()`, not `len(...)`, so legitimate warnings do not halt the operation.

## Wrong schema package

`provider/schema`, `resource/schema`, and `datasource/schema` are distinct packages with similarly named types. Importing the wrong one for the context is a common early compile error.

## Naming collisions with List Resources

Naming a plural data source with `List`/`ListResource` collides with the actual List Resources primitive. Use the `Collection` convention for plural data sources and reserve `List` for the real feature. See `naming-conventions.md`.