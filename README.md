# Terraform Provider for Ona

This repository contains the Terraform provider for Ona. It is built with the
[Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).

The provider is intended to manage Ona projects, runners, runner environment
classes, security policies, organization policies, product Automations, teams,
groups, and AI budget policies.

The provider currently includes an import/read-only `ona_runner` resource for
the first brownfield dogfood workflow.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://go.dev/doc/install) >= 1.25.8

## Building the Provider

```shell
go install .
```

## Developing the Provider

Download dependencies:

```shell
go mod download
```

Run unit tests:

```shell
go test ./...
```

Run acceptance tests:

```shell
TF_ACC=1 make testacc
```

Generate documentation:

```shell
make generate
```

## Releasing

Beta releases are published from semver prerelease tags such as
`v0.1.0-beta.1`. See [docs/release.md](docs/release.md) for the release
checklist, required secrets, and registry smoke test.

## Local Terraform Override

To run Terraform against a locally built provider binary, install the provider:

```shell
go install .
```

Then configure a Terraform CLI development override for `gitpod-io/ona` pointing at
the directory containing the built binary.

## Import Existing Resources

The Terraform-native brownfield workflow is:

1. discover existing Ona resources through the provider,
2. create Terraform import blocks,
3. run Terraform config generation,
4. apply the imports, and
5. verify that the resulting plan is a no-op.

The provider still needs real Ona resources with import state, resource identity,
read, and list-resource support before Terraform can perform that full workflow
through the provider protocol. Until then, this repository includes helper code
that prepares Terraform-native import blocks and generated configuration for the
same workflow.

See [examples/import.md](examples/import.md) for the full workflow and flags.
