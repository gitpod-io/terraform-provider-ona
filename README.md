# Terraform Provider for Ona

This repository contains the Terraform provider for Ona. It is built with the
[Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).

The provider is intended to manage Ona projects, runners, runner environment
classes, security policies, organization policies, product Automations, teams,
groups, and AI budget policies.

The module includes the copied API client subset under `internal/api/go` so it
can build without importing private monorepo Go modules.

The provider currently includes:

- `ona_project` for managing projects.
- `ona_runner` for managing runner registrations.
- `ona_environment_class` for managing runner environment classes.
- `ona_scm_integration` for managing runner SCM integrations.
- `ona_security_policy` and `ona_security_policies` for managing and listing
  runtime security policies.
- `ona_organization_policies` for managing organization-level policy settings.
- `ona_runner_token` for issuing runner registration tokens.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.11
- [Go](https://go.dev/doc/install) >= 1.25.8

## Building the Provider

```shell
make build
```

## Developing the Provider

Download dependencies:

```shell
go mod download
```

Run unit tests:

```shell
make test
```

Run acceptance tests:

```shell
TF_ACC=1 go test -v -cover -timeout 120m ./...
```

Generate documentation:

```shell
make generate
```

## Releasing

Beta releases are published from semver prerelease tags such as
`v0.1.0-beta.1`. See [docs/release.md](docs/release.md) for the release
checklist, required secrets, and registry smoke test.

## Local Terraform Dev Loop

To run Terraform against a locally built provider binary and the local dev loop
workspace:

```shell
mkdir -p .bin
go build -o .bin/terraform-provider-ona .
cat > terraformrc <<EOF
provider_installation {
  dev_overrides {
    "gitpod-io/ona" = "${PWD}/.bin"
    "ona-com/ona"  = "${PWD}/.bin"
  }
  direct {}
}
EOF
ONA_TOKEN="<api-token>" \
TF_CLI_CONFIG_FILE="${PWD}/terraformrc" \
terraform -chdir=dev/local-devloop plan -input=false
```

This builds the provider, configures a temporary Terraform CLI development
override for `gitpod-io/ona` and `ona-com/ona`, and runs `terraform plan` by
default.

Run a different Terraform command by changing the final Terraform invocation:

```shell
ONA_TOKEN="<api-token>" \
TF_CLI_CONFIG_FILE="${PWD}/terraformrc" \
terraform -chdir=dev/local-devloop apply -auto-approve -input=false
```

## Import Existing Resources

The Terraform-native brownfield workflow is:

1. discover existing Ona resources through the provider,
2. create Terraform import blocks,
3. run Terraform config generation,
4. apply the imports, and
5. verify that the resulting plan is a no-op.

The provider supports project, runner registration, runner environment class,
and runner SCM integration resources. Resource families without native
Terraform resources still need provider implementations before Terraform can
import them directly, so this repository includes helper code that prepares
Terraform-native import blocks and generated configuration only for
registered/importable provider resource types.

See [examples/import.md](examples/import.md) for the full workflow and flags.
