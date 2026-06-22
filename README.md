# Terraform Provider for Ona

This repository contains the Terraform provider for Ona. It is built with the
[Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).

The provider is intended to manage Ona projects, runners, runner environment
classes, security policies, organization policies, product Automations, teams,
groups, and AI budget policies.

The repository is currently bootstrapped from HashiCorp's framework template.
The example resource, data source, ephemeral resource, action, and function are
placeholders and should be replaced with real Ona API-backed implementations.

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

## Local Terraform Override

To run Terraform against a locally built provider binary, install the provider:

```shell
go install .
```

Then configure a Terraform CLI development override for `ona/ona` pointing at
the directory containing the built binary.
