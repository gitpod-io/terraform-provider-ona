# Release Process

The provider source lives in this repository as the standalone Go module
`github.com/gitpod-io/terraform-provider-ona`. The Terraform Registry provider
source address remains `registry.terraform.io/gitpod-io/ona`, so user
configuration should continue to use `source = "gitpod-io/ona"`.

The copied API client subset lives under `internal/api/go`. Refresh it from the
monorepo source intentionally and keep `sync/api-subset.manifest` in sync with
the copied files.

Beta releases intentionally publish Linux artifacts only:

- `linux_amd64`
- `linux_arm64`

Add macOS and Windows packages when beta usage moves beyond Ona-hosted Linux
environments.

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
version/STABLE_VERSION -> 0.2.0
version/BETA_VERSION   -> 0.3.0-beta
```

`version/STABLE_VERSION` is exact bare SemVer for manually published stable
releases. `version/BETA_VERSION` is a beta line, not an exact release version.
It omits the numeric beta suffix; Git tags carry the auto-incrementing suffix.

For example, if `version/BETA_VERSION` contains `0.3.0-beta` and the latest
matching tag is `v0.3.0-beta.6`, the next beta publish creates
`v0.3.0-beta.7`.

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
release, and starts Registry verification.

Published releases are GitHub releases in
`gitpod-io/terraform-provider-ona`. After creating the GitHub release, CI starts
the separate `Verify Terraform Registry` workflow. That workflow waits for
Terraform Registry ingestion and runs `terraform init` on Linux amd64 and Linux
arm64 without blocking the main build.

After publishing a stable version, prepare the next beta-line PR before the next
beta publish attempt if the current beta line is not greater than the latest
stable tag. The release workflow rejects versions that are not greater than the
existing SemVer tags.

## Local Verification

Before merging a release-prep PR, run:

```shell
make build
make test
make generate
git diff --exit-code
```

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
`terraform-provider-ona/0.3.0-beta.7` or `terraform-provider-ona/0.2.0`.

Local publishing is not supported. The publish scripts are CI entrypoints and
fail unless GitHub Actions runs them from `refs/heads/main`.

Before expecting Terraform Registry ingestion, confirm the Registry provider is
registered and reachable:

```shell
REGISTRY_PROVIDER=gitpod-io/ona scripts/check-registry-provider.sh
```

After Terraform Registry ingestion, run a registry smoke test:

```shell
tmpdir="$(mktemp -d)"
cat >"${tmpdir}/main.tf" <<'EOF'
terraform {
  required_providers {
    ona = {
      source  = "gitpod-io/ona"
      version = "= 0.2.0"
    }
  }
}

provider "ona" {}
EOF

terraform -chdir="${tmpdir}" init
```

The smoke test passes when Terraform downloads the provider from the Registry
and exits successfully.

## Customer Beta Configuration

```hcl
terraform {
  required_providers {
    ona = {
      source  = "gitpod-io/ona"
      version = "= 0.2.0-beta.1"
    }
  }
}

provider "ona" {}
```
