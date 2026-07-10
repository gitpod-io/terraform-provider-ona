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

## CI

Pull requests run the branch build. It checks formatting, generated docs, lint,
tests, normal build output, and unsigned release artifact packaging.

Pushes to `main` run the main build and publish a prerelease GitHub release after
the provider checks pass. The manual `Build main` workflow can also publish a
specific prerelease version.

Published prereleases are GitHub releases in
`gitpod-io/terraform-provider-ona`. After creating the GitHub release, CI starts
the separate `Verify Terraform Registry` workflow. That workflow waits for
Terraform Registry ingestion and runs `terraform init` on Linux amd64 and Linux
arm64 without blocking the main build.

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
make release-snapshot RELEASE_SNAPSHOT_VERSION=0.1.0-beta.1
```

The snapshot command writes Linux artifacts to `dist/release-snapshot/` and
verifies the artifact inventory, zip contents, registry manifest, and checksums.

## Manual Publish

Run the preflight first:

```shell
VERSION=v0.1.0-beta.1 \
RELEASE_REPOSITORY=gitpod-io/terraform-provider-ona \
GPG_PRIVATE_KEY="$(cat terraform-provider-ona-private.asc)" \
GPG_FINGERPRINT="<fingerprint>" \
scripts/preflight-publish-release.sh
```

Publish the GitHub release:

```shell
VERSION=v0.1.0-beta.1 \
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
      version = "= 0.1.0-beta.1"
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
      version = "= 0.1.0-beta.1"
    }
  }
}

provider "ona" {}
```

Stable release promotion remains a separate follow-up from the prerelease CI
path.
