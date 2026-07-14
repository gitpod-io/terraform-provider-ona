---
name: release-prep
description: Prepare explicit beta or stable release PRs for terraform-provider-ona by selecting the next provider version, updating VERSION and CHANGELOG.md, validating release metadata, and describing the manual publish step.
---

# Release Prep

Use this skill when asked to prepare, review, or reason about an Ona Terraform provider release PR.

## Release Model

- `VERSION` is the reviewed release intent and contains bare SemVer, for example `0.2.0-beta.1`.
- GitHub release tags add the leading `v`, for example `v0.2.0-beta.1`.
- `terraform-registry-manifest.json` version is the Registry manifest schema version, not the provider version.
- Pushes to `main` run CI but do not publish. Publishing is a manual `Build main` workflow run with `publish_release=true` after the release-prep PR merges.

## Workflow

1. Inspect existing tags and releases. Use the highest SemVer tag as the lower bound; use the most recent published tag as the diff base for changelog review when those differ.
2. Compare the release diff against provider compatibility policy and classify the bump:
   - beta/prerelease: use a prerelease suffix such as `0.2.0-beta.1`.
   - stable patch/minor/major: use plain SemVer such as `0.2.0`.
3. Update `VERSION` to bare SemVer with no leading `v`.
4. Update the first `CHANGELOG.md` heading to the same version.
5. Run `scripts/validate-release-version.sh`.
6. For a release-prep PR, keep the PR focused on release metadata unless the user asked for release automation changes too.
7. After merge, run the `Build main` workflow manually with `publish_release=true`; the workflow reads `VERSION`, creates the `v...` release, and starts Registry verification.
8. After a release is published, prepare the next release-prep PR before the next publish attempt so `VERSION` moves beyond the published tag.

## Guardrails

- Do not publish on every merge to `main`.
- Do not release a version lower than an existing SemVer tag, even if that lower version has a larger run number.
- Do not put the leading `v` in `VERSION`.
- Do not treat prereleases as automatically selected by Terraform version ranges; users should pin beta versions explicitly.
