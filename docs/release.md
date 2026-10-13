# Release Process

The provider source lives in this repository as the standalone Go module
`github.com/gitpod-io/terraform-provider-ona`. The Terraform Registry provider
source address remains `registry.terraform.io/gitpod-io/ona`, so user
configuration should continue to use `source = "gitpod-io/ona"`.

The copied API client subset lives under `internal/api/go`. Refresh it from the
monorepo source intentionally and keep `sync/api-subset.manifest` in sync with
the copied files.

Provider releases currently publish Linux artifacts only:

- `linux_amd64`
- `linux_arm64`

macOS and Windows packages are not currently available.

## Prerequisites

- Terraform Registry provider registered under the `gitpod-io` namespace and
  `ona` type.
- A dedicated GPG signing key whose public key is registered in the Terraform
  Registry provider settings.
- GitHub Actions release credentials with `contents: write` permission for
  `gitpod-io/terraform-provider-ona`.
- A `beta` GitHub environment for automatic beta publishing. If release
  credentials are environment-scoped, mirror the release secrets from the stable
  environment; keep approval gates on the stable environment only.
- Local verification tools: Go, Terraform, `jq`, `sha256sum`, `zip`, and
  `zipinfo`.

Do not upload the private key to Terraform Registry, and do not commit exported
key files.

## Release Versions

The provider tracks stable and beta release intent separately:

```text
version/STABLE_VERSION -> 0.5.0
version/BETA_VERSION   -> 0.5.0-beta
```

`version/STABLE_VERSION` is exact bare SemVer for manually published stable
releases. `version/BETA_VERSION` is a beta line, not an exact release version.
It omits the numeric beta suffix; Git tags carry the auto-incrementing suffix.

For example, if `version/BETA_VERSION` contains `0.5.0-beta` and the latest
matching tag is `v0.5.0-beta.1`, the next beta publish creates
`v0.5.0-beta.2`.

Stable release-prep PRs update both `version/STABLE_VERSION` and the top
`CHANGELOG.md` heading. Beta-line PRs update only `version/BETA_VERSION`.
Run the release metadata checks before merging release-flow changes:

```shell
scripts/validate-release-version.sh --channel stable
scripts/validate-release-version.sh --channel beta
```

## CI

Pull requests run the branch build. It checks formatting, generated docs, lint,
tests, normal build output, and unsigned release artifact packaging.

Pushes to `main` run the main build and automatically publish the next beta
release from `version/BETA_VERSION`. The workflow reads the beta line, finds
the latest matching `v...-beta.N` tag, publishes the next tag, and starts
Registry verification.

Stable releases remain manual. To publish a stable release, merge a stable
release-prep PR and then run the manual `Build main` workflow from `main` with
`release_channel=stable`. The workflow reads `version/STABLE_VERSION`,
validates the matching `CHANGELOG.md` heading, publishes the stable GitHub
release, and starts Registry verification. Updating `STABLE_VERSION` alone does not
publish the stable release.

Published releases are GitHub releases in
`gitpod-io/terraform-provider-ona`. After creating the GitHub release, CI starts
the separate `Verify Terraform Registry` workflow. That workflow waits for
Terraform Registry ingestion and runs `terraform init` on Linux amd64 and Linux
arm64 without blocking the main build.

After publishing a stable version, prepare the next beta-line PR before the next
beta publish attempt if the current beta line is not greater than the latest
stable tag. The release workflow rejects versions that are not greater than the
existing SemVer tags. After publishing `v0.5.0`, advance the beta line in a
separate PR before another beta publish attempt, for example to `0.6.0-beta`
if that is the next intended development line.

## Local Verification

Before merging a release-prep PR, run:

```shell
make build
make test-unit
make generate
git diff --exit-code
```

`make test` also runs credentialed acceptance tests. Run it or `make test-acc`
only when explicitly authorized and the required Ona credentials are available.

Build unsigned local release artifacts:

```shell
make release-snapshot RELEASE_SNAPSHOT_VERSION="$(cat version/STABLE_VERSION)"
```

The snapshot command writes Linux artifacts to `dist/release-snapshot/` and
verifies the artifact inventory, zip contents, registry manifest, and checksums.

## Publishing

Publish only through GitHub Actions from `main`. Beta publishing happens
automatically on pushes to `main`. Stable publishing happens through the manual
`Build main` workflow from `main` with `release_channel=stable`.

The release jobs cross-compile the Linux provider binaries, write Terraform
Registry artifacts, sign `SHA256SUMS`, create the GitHub release, download the
published assets, and verify them again.

Release binaries embed the resolved release version in provider metadata and
the default Ona API `User-Agent`, for example
`terraform-provider-ona/0.5.0`.

Local publishing is not supported. The publish scripts are CI entrypoints and
fail unless GitHub Actions runs them from `refs/heads/main`.

Before expecting Terraform Registry ingestion, confirm the Registry provider is
registered and reachable:

```shell
REGISTRY_PROVIDER=gitpod-io/ona scripts/check-registry-provider.sh
```

After the stable GitHub release and all signed artifacts are available, manually
trigger ingestion through the provider's Terraform Registry publishing controls.
Do not rely on webhook delivery for this promotion. Confirm the available manual
publishing control with the provider maintainer when carrying out the release.
The Registry ingests the GitHub release artifacts, not `version/STABLE_VERSION`.

The `Verify Terraform Registry` workflow waits for ingestion and verifies
installation; it does not request a Registry pull. If verification times out
before the manual trigger completes, rerun it with `source=gitpod-io/ona` and
`version=0.5.0` after triggering ingestion.

After Terraform Registry ingestion, run a registry smoke test:

```shell
tmpdir="$(mktemp -d)"
cat >"${tmpdir}/main.tf" <<'EOF'
terraform {
  required_providers {
    ona = {
      source  = "gitpod-io/ona"
      version = "= 0.5.0"
    }
  }
}

provider "ona" {}
EOF

terraform -chdir="${tmpdir}" init
```

The smoke test passes when Terraform downloads the provider from the Registry
and exits successfully.

## Customer Stable Configuration

```hcl
terraform {
  required_providers {
    ona = {
      source  = "gitpod-io/ona"
      version = "= 0.5.0"
    }
  }
}

provider "ona" {}
```
