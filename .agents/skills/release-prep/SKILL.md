---
name: release-prep
description: Prepare beta-line or stable release PRs for terraform-provider-ona by selecting the next provider version, updating version/STABLE_VERSION, version/BETA_VERSION, and CHANGELOG.md as appropriate, validating release metadata, and describing the publish step.
---

# Release Prep

Use this skill when asked to prepare, review, or reason about an Ona Terraform provider release PR.

## Release Model

- `version/STABLE_VERSION` is the reviewed stable release intent and contains exact bare SemVer, for example `0.2.0`.
- `version/BETA_VERSION` is the reviewed beta release line and contains a bare beta line without the numeric suffix, for example `0.3.0-beta`.
- GitHub release tags add the leading `v`, for example `v0.2.0` or `v0.3.0-beta.7`.
- `terraform-registry-manifest.json` version is the Registry manifest schema version, not the provider version.
- Pushes to `main` run CI and automatically publish the next beta tag from `version/BETA_VERSION`.
- Stable publishing is a manual `Build main` workflow run from `main` with `release_channel=stable` after the stable release-prep PR merges.
- Local publishing is not supported; the publish scripts are invoked by GitHub Actions on `main`.

## Workflow

1. Inspect existing tags and releases. Use the highest SemVer tag as the lower bound; use the most recent published tag as the diff base for changelog review when those differ.
2. For a beta-line PR, update only `version/BETA_VERSION` to the intended beta line, for example `0.3.0-beta`.
3. For a stable release-prep PR, update `version/STABLE_VERSION` to exact bare SemVer and update the first `CHANGELOG.md` heading to the same version.
4. Run both release metadata checks:
   - `scripts/validate-release-version.sh --channel stable`
   - `scripts/validate-release-version.sh --channel beta`
5. For a release-prep PR, keep the PR focused on release metadata unless the user asked for release automation changes too.
6. After a beta-line PR merges, the next push to `main` publishes the next computed `v...-beta.N` tag.
7. After a stable release-prep PR merges, run the `Build main` workflow manually from `main` with `release_channel=stable`.
8. After a stable release is published, prepare the next beta-line PR before the next beta publish attempt if the current beta line is no longer greater than the latest stable tag.

## Guardrails

- Do not publish stable releases on every merge to `main`.
- Do not publish from release-prep branches or local workstations.
- Do not release a version lower than an existing SemVer tag, even if that lower version has a larger run number.
- Do not put the leading `v` in `version/STABLE_VERSION` or `version/BETA_VERSION`.
- Do not put the numeric beta suffix in `version/BETA_VERSION`; tags carry the auto-incrementing `.N` suffix.
- Do not treat prereleases as automatically selected by Terraform version ranges; users should pin beta versions explicitly.
