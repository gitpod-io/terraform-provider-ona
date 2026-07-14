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
- GitHub credentials with `contents: write` permission for
  `gitpod-io/terraform-provider-ona`.
- Local tools: Go, Terraform, `gh`, `gpg`, `jq`, `sha256sum`, `zip`, and
  `zipinfo`.

Do not upload the private key to Terraform Registry, and do not commit exported
key files.

## Release Version

The provider version is reviewed in `VERSION`. Keep it as bare SemVer without a
leading `v`, for example:

```text
0.2.0-beta.1
```

Use prerelease suffixes such as `-beta.1` for explicit beta releases. Use plain
SemVer such as `0.2.0` for stable releases. GitHub release tags add the leading
`v`, so `VERSION=0.2.0-beta.1` publishes tag `v0.2.0-beta.1`.

Release-prep PRs update both `VERSION` and the top `CHANGELOG.md` heading. Run
the release metadata check before merging:

```shell
scripts/validate-release-version.sh
```

## CI

Pull requests run the branch build. It checks formatting, generated docs, lint,
tests, normal build output, and unsigned release artifact packaging.

Pushes to `main` run the main build without publishing. To publish a beta or
stable release, merge a release-prep PR and then run the manual `Build main`
workflow with `publish_release=true`. The workflow reads `VERSION`, validates
the release metadata, publishes the matching GitHub release, and starts Registry
verification.

Published releases are GitHub releases in
`gitpod-io/terraform-provider-ona`. After creating the GitHub release, CI starts
the separate `Verify Terraform Registry` workflow. That workflow waits for
Terraform Registry ingestion and runs `terraform init` on Linux amd64 and Linux
arm64 without blocking the main build.

After publishing a version, prepare the next release-prep PR before the next
publish attempt. The release workflow rejects versions that are not greater than
the existing SemVer tags.

## Local Verification

Before publishing locally, run:

```shell
make build
make test
make generate
git diff --exit-code
```

Build unsigned local release artifacts:

```shell
make release-snapshot RELEASE_SNAPSHOT_VERSION="$(cat VERSION)"
```

The snapshot command writes Linux artifacts to `dist/release-snapshot/` and
verifies the artifact inventory, zip contents, registry manifest, and checksums.

## Manual Publish

Prefer the manual `Build main` workflow with `publish_release=true`. Use the
local scripts only when publishing from a trusted operator workstation.

Run the preflight first:

```shell
VERSION="v$(cat VERSION)" \
RELEASE_REPOSITORY=gitpod-io/terraform-provider-ona \
GPG_PRIVATE_KEY="$(cat terraform-provider-ona-private.asc)" \
GPG_FINGERPRINT="<fingerprint>" \
scripts/preflight-publish-release.sh
```

Publish the GitHub release:

```shell
VERSION="v$(cat VERSION)" \
RELEASE_REPOSITORY=gitpod-io/terraform-provider-ona \
GPG_PRIVATE_KEY="$(cat terraform-provider-ona-private.asc)" \
GPG_FINGERPRINT="<fingerprint>" \
scripts/publish-release.sh
```

`scripts/publish-release.sh` cross-compiles the Linux provider binaries, writes
Terraform Registry artifacts, signs `SHA256SUMS`, creates the GitHub release,
downloads the published assets, and verifies them again.

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
      version = "= 0.2.0-beta.1"
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
