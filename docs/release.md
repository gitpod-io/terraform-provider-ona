# Release Process

This repository uses a lean GitHub Releases plus GoReleaser flow for beta
provider releases. It intentionally avoids release promotion infrastructure
until the provider has real users and a stronger need for staged channels.

Beta releases intentionally publish Linux artifacts only:

- `linux_amd64`
- `linux_arm64`

That keeps the first release loop small and aligned with Ona environments. Add
macOS and Windows packages when beta usage moves beyond Ona-hosted Linux
environments.

## Release Prerequisites

Before the first beta release:

- Confirm the Terraform provider source address. The provider serves
  `registry.terraform.io/gitpod-io/ona`, so user configuration should use
  `source = "gitpod-io/ona"`.
- Register the provider in the Terraform Registry under the same namespace and
  type.
- Add the GPG public key to the Terraform Registry provider settings.
- Add these GitHub Actions secrets:
  - `GPG_PRIVATE_KEY`: armored private key used to sign checksum files.
  - `GPG_PASSPHRASE`: passphrase for the signing key.

## Signing Key Setup

Provider releases must be signed. Use a dedicated release signing key and keep
the private key out of the repository.

Create the key on a trusted local machine:

```shell
gpg --full-generate-key
```

Use RSA, 4096 bits, and a passphrase. Then find the key fingerprint:

```shell
gpg --list-secret-keys --keyid-format=long
```

Export the public key for the Terraform Registry:

```shell
gpg --armor --export "<fingerprint>" > terraform-provider-ona-public.asc
```

Export the private key for GitHub Actions:

```shell
gpg --armor --export-secret-keys "<fingerprint>" > terraform-provider-ona-private.asc
```

Add the public key to the Terraform Registry public namespace:

1. Open the `gitpod-io` public namespace in HCP Terraform.
2. Add a new GPG key.
3. Paste the full contents of `terraform-provider-ona-public.asc`.

Add the private signing material to the GitHub repository:

1. Open `gitpod-io/terraform-provider-ona` in GitHub.
2. Go to `Settings` -> `Secrets and variables` -> `Actions`.
3. Add `GPG_PRIVATE_KEY` with the full contents of
   `terraform-provider-ona-private.asc`.
4. Add `GPG_PASSPHRASE` with the key passphrase.

Do not upload the private key to Terraform Registry, and do not commit either
exported key file.

## Beta Release Flow

1. Merge the release candidate to `main`.
2. Ensure the `Tests` and `Release Checks` workflows are green.
3. Create and push an annotated beta tag:

   ```shell
   git checkout main
   git pull --ff-only origin main
   git tag -a v0.1.0-beta.1 -m "v0.1.0-beta.1"
   git push origin v0.1.0-beta.1
   ```

4. Wait for the `Release` workflow to finish. It creates a GitHub prerelease
   with Terraform Registry-compatible provider archives, checksums, checksum
   signature, and registry manifest.
5. After Terraform Registry ingestion, run the `Registry Init Check` workflow:
   - `source`: `gitpod-io/ona`
   - `version`: `0.1.0-beta.1`

The registry check waits for the version to appear in the Terraform Registry,
then runs `terraform init` on Linux amd64 and Linux arm64 runners.

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

## Stable Release Flow

Use the same process with a stable semver tag, for example `v0.1.0`, after the
beta has been validated. Stable release tags should only be created from
commits that have already passed beta release checks.

## Local Release Packaging Check

Install GoReleaser and run:

```shell
make release-snapshot
```

This validates the GoReleaser configuration and builds local snapshot archives
without publishing or signing them.
