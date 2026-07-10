# Agent Guide

This repository is the standalone Go module for the Ona Terraform provider. It
uses the [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
and keeps a copied API client subset under `internal/api/go` so the provider can
build without private monorepo modules.

## Important Paths

- `internal/provider/**`: provider implementation, resource/data source registration, and provider tests.
- `internal/client/**`: Ona API client wrapper used by provider code.
- `internal/api/go/**`: copied/generated API subset. Do not hand-edit for lint-only or style changes.
- `docs/**`: generated Terraform provider documentation.
- `examples/**`: Terraform examples consumed by docs generation.
- `scripts/**`: import helper and release-related tooling.
- `dev/local-devloop/**`: local Terraform dev loop.
- `tools/**`: separate Go module used by Terraform documentation generation tooling.

## Core Commands

This repo uses `make`; see [GNUmakefile](GNUmakefile) for the source of truth.

- Install dependencies: `make install-dependencies`.
- Build: `make build`.
- Unit tests: `make test-unit`.
- Full tests: `make test`.
- Format: `make fmt`.
- Lint: `make lint`.
- Generate: `make generate`.
- Check generated output: `git diff --exit-code`.
- Acceptance tests: `make test-acc`.

CI downloads dependencies for both modules, runs `make fmt`, runs
`make generate && git diff --exit-code`, then runs `make lint`, `make test`,
and `make build`.

## Development Rules

- Prefer the `make` targets when they cover the workflow.
- Run `make generate` when provider schemas, examples, generated docs, or codegen inputs change.
- Keep Terraform docs and examples aligned. Docs are generated from provider schema and files under `examples/**`.
- Do not commit real `ONA_TOKEN` values, private keys, Terraform state, generated local provider override files, or release signing material.
- Treat acceptance tests and live Terraform plans as credentialed operations. Run them only when explicitly requested and credentials are available.
- Preserve user changes already present in the worktree. Check `git status --short` before editing.

## Reviews

For code reviews, self-reviews, PR reviews, or review-style checks, use the
repository review skill at `.agents/skills/review/SKILL.md`.
