# Provider State Safety (repo review findings)

Canonical state-safety rules distilled from real review findings on Ona Terraform provider PRs (#26527, #26532, #26586). Each is a repo-verified instance of a failure mode from `pitfalls.md`, with the fixed code cited so you can copy the pattern. Check every resource against this list before opening a provider PR.

## 1. Persist the remote ID into state immediately after create

This is the canonical detailed rule for immediate ID persistence. The instant the create RPC succeeds, set the ID on the model and write state — before any subsequent fallible step (post-create update call, readback, model population, planned-input preservation, final `State.Set`). Two separate reviews flagged the same bug:

> "Create can return an error from the post-create disable call before any Terraform state containing the new environment class ID has been written. That can orphan an enabled remote environment class and a retry can create a duplicate." (#26532)

> "After `CreateSecurityPolicy` succeeds, this resource does not persist the returned policy ID before doing the full model conversion and final `State.Set`. Terraform can lose track of an already-created remote policy and create a duplicate on the next apply." (#26586)

Done right: `Create` in `internal/provider/runner/environment_class_resource.go` sets `data.ID = types.StringValue(result.Msg.GetId())` and calls `resp.State.Set(ctx, &data)` directly after `CreateEnvironmentClass` returns, before the fallible disable call and the readback that produces the final refreshed state.

## 2. Update must handle removed blocks, not just changed ones

An optional block that exists in prior state but is absent from the plan means the user deleted it. Omitting the field from the update request leaves the remote value in place, and the next refresh reintroduces drift:

> "When the planned config omits `update_window`, this update path omits `UpdateWindow` entirely instead of sending the empty message that the API uses to clear a custom window. Removing the block from Terraform will leave the remote runner on the old window, then the next refresh can reintroduce drift and repeated plans." (#26527)

Read prior state in `Update` (`req.State.Get`) alongside the plan, and when a block was removed, send the API's explicit clear (an empty message, null, or whatever the API defines), never an omitted field. Done right: `Update` in `internal/provider/runner/runner_resource.go` loads `prior` from state and `updateRunnerRequest` sends an empty `v1.UpdateWindow{}` when the block was removed. Cover create-with-block then remove-block in an acceptance test.

## 3. Schema defaults must satisfy API validation

A default the API rejects turns "optional attribute" into a guaranteed apply-time failure:

> "The provider schema makes description optional with an empty-string default, but createEnvironmentClassRequest always sends that value to an API field with min_len=3. Users can omit description as the schema/docs allow and then hit an apply-time API validation error." (#26532)

For every defaulted attribute, check the API's validation rules (proto `min_len`, enums, formats) and pick a default that passes them — or make the attribute required. Done right: `environment_class_resource.go` defaults `description` to `defaultEnvironmentClassDescription` ("Environment class managed by Terraform."), covered by `TestAccEnvironmentClassResourceDefaultDescription` in `internal/provider/runner_configuration_resource_test.go`. Always add a test that creates the resource without setting the defaulted attribute.

## 4. Enforce cross-attribute rules at plan time in ValidateConfig

Validating attributes independently lets invalid combinations reach the API:

> "`ValidateConfig` validates the provider and configuration independently, so an `aws_ec2` runner with no `configuration.region` still reaches `CreateRunner` with an empty region. Users can apply an invalid AWS runner and only discover the problem through backend behavior." (#26527)

When one attribute's validity depends on another (provider type implies a required region), validate the combination in `ValidateConfig` and cover both directions in tests. Done right: `ValidateConfig` in `runner_resource.go` reads both `runner_provider` and `configuration`, and `validateRegion` requires `configuration.region` for `aws_ec2` while allowing `gcp` to omit it — tested by `TestAccRunnerResourceRequiresRegionForAWSEC2` and `TestAccRunnerResourceAllowsGCPWithoutRegion` in `internal/provider/runner_resource_test.go`. Skip the check while values are unknown; re-validate known values only.

## 5. File moves and deletions must update every consumer in the same PR

Not state machinery, but flagged repeatedly on the same PR series — deleting or renaming provider files silently broke build and release plumbing:

> "This PR removes the `release-snapshot` Leeway target and deletes `scripts/publish-release.sh`, but the existing provider release workflow still calls both. That will make release packaging fail immediately." (#26527)

Before removing or moving provider files, grep for consumers — `.github/workflows`, docs under `docs/`, examples under `examples/`, scripts under `scripts/`, and generated provider docs — and update them in the same PR, or the breakage lands only after merge.
