# Contributing

We are not accepting external pull requests for the Terraform Provider for Ona
at this time. For questions or feedback, contact us through
[Ona support](https://ona.com/support).

You are still welcome to clone the repository or save the files locally. This
guide describes how to set up the repository, make local changes, and run the
checks used by continuous integration (CI).

Report security vulnerabilities through the process in
[SECURITY.md](SECURITY.md), not through a public issue.

## Development environment

The easiest way to get started is to open the repository in
[Ona](https://ona.com/) or run the included
[dev container](.devcontainer/) locally with
[VS Code Dev Containers](https://code.visualstudio.com/docs/devcontainers/containers)
or another compatible editor. The dev container includes Go, Terraform,
`make`, `golangci-lint`, and `shellcheck`.

For a manual setup, install the tool versions defined by the repository:

- [Go](https://go.dev/doc/install), using the version in [go.mod](go.mod)
- [Terraform](https://developer.hashicorp.com/terraform/install), version
  1.14 or later
- [GNU Make](https://www.gnu.org/software/make/)
- [golangci-lint](https://golangci-lint.run/)
- [ShellCheck](https://www.shellcheck.net/)

Download dependencies for the provider and documentation tooling:

```shell
make install-dependencies
```

## Repository structure

| Path | Description |
| --- | --- |
| `internal/provider/` | Provider, resource, data source, and ephemeral resource implementations and tests |
| `internal/client/` | Ona API client wrapper used by the provider |
| `api/public-clients/go/` | Copied generated public API client; do not hand-edit it for lint-only or style changes |
| `examples/` | Terraform examples used to generate provider documentation |
| `docs/` | Generated Terraform Registry documentation |
| `templates/` | Source templates used by documentation generation |
| `scripts/` | Import, validation, and release tooling |
| `dev/local-devloop/` | Local Terraform configuration for exercising a development build |
| `tools/` | Separate Go module containing documentation generation tools |

## Making local changes

1. Make a focused change and add or update tests where behavior changes.
2. If you change a provider schema, documentation template, or example, run
   `make generate` and commit the generated output.
3. Run the relevant checks described below.

Keep examples and generated documentation aligned. The documentation generator
only reads example files in the locations documented in
[examples/README.md](examples/README.md).

## Formatting, generation, and validation

Format Go and Terraform files:

```shell
make fmt
```

Regenerate provider documentation when schemas, templates, or examples change:

```shell
make generate
```

Confirm that formatting and generation leave no uncommitted differences:

```shell
git diff --exit-code
```

Run linting, unit tests, and a build:

```shell
make lint
make test-unit
make build
```

The full test target also runs acceptance tests:

```shell
make test
```

Treat acceptance tests and live Terraform plans as credentialed operations.
Run `make test-acc` or the configuration under `dev/local-devloop/` only when
the change requires it, you have been explicitly authorized, and the required
Ona credentials are available. Never commit tokens, private keys, Terraform
state, provider override files, or release signing material.

## License

By contributing, you agree that your contributions will be licensed under the
[Mozilla Public License 2.0](LICENSE).
