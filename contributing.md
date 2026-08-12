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

Use the included dev container, or install the Go version in `go.mod`,
Terraform 1.14 or later, GNU Make, `golangci-lint`, and ShellCheck. Then run
`make install-dependencies`.

## Repository structure

| Path | Description |
| --- | --- |
| `internal/provider/` | Provider, resource, data source, and ephemeral resource implementations and tests |
| `internal/managementclient/` | Provider-facing wrapper around generated Ona API services |
| `examples/` | Terraform examples used to generate provider documentation |
| `docs/` | Generated Terraform Registry documentation |
| `templates/` | Source templates used by documentation generation |
| `scripts/` | Validation and release tooling |
| `dev/local-devloop/` | Local Terraform configuration for exercising a development build |
| `dev/local-importloop/` | Local Terraform Query configuration for exercising all supported bulk imports |
| `tools/` | Separate Go module containing documentation generation tools |

## Making local changes

1. Make a focused change and add or update tests where behavior changes.
2. If you change a provider schema, documentation template, or example, run
   `make generate` from the repository root
   and commit the generated output.
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
git diff --exit-code
```

Confirm that formatting and generation leave no uncommitted differences:

```shell
git diff --exit-code
```

Run unit tests, acceptance tests, and a build:

```shell
make test-unit
make test-acc
make build
```

The checked-in acceptance suite uses hermetic test servers and is required in
CI. Treat the configurations under `dev/local-devloop/` and
`dev/local-importloop/` as credentialed operations. Run them against a live Ona
API only when the change requires it,
you have been explicitly authorized, and the required credentials are
available. Never commit tokens, private keys, Terraform state, provider
override files, or release signing material.

## License

By contributing, you agree that your contributions will be licensed under the
[Mozilla Public License 2.0](LICENSE).
