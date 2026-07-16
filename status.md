# PDE-1057 implementation status

## Current status

The workflow automation provider work is implemented and locally validated. The change adds the `ona_automation` managed resource and the `ona_workflows` collection data source, using the existing generated `WorkflowService` client.

The implementation was checked against `/workspaces/gitpod-next` at commit `126e0b41dbf01a2d9642b489916e50f8c3a0dbec`. There are no known implementation blockers. The changes remain uncommitted in the local worktree; no pull request has been created.

## Work completed

### Managed workflow resource

Added `ona_automation` with support for:

- create, read, update, delete, and direct import by workflow ID;
- workflow names and descriptions;
- ordered manual, scheduled, and pull-request triggers;
- project, repository, agent, and trigger-derived execution contexts;
- ordered task, agent, and pull-request action steps;
- execution limits and optional per-action duration limits;
- user and service-account executors;
- disabled state;
- computed creator, executor, webhook URL, and timestamps;
- drift detection, remote deletion, and graceful deletion in progress.

The resource deliberately rejects imports or reads that would be lossy. This includes workflows using report actions, report steps, workflow-level agent or Codex settings, and legacy pull-request triggers without a webhook or integration ID.

### Collection data source

Added `ona_workflows` backed by `ListWorkflows`. It supports:

- workflow ID, creator ID, search, execution-phase, failed-since, and tri-state disabled filters;
- validation matching the backend's UUID, cardinality, timestamp, and mutually exclusive filter rules;
- complete pagination with a bounded page size;
- deterministic sorting by workflow ID;
- summary results containing metadata, executor, creator, deletion state, and timestamps;
- failure without publishing partial Terraform state.

### Provider integration and documentation

- Registered the resource and data source in `internal/provider/provider.go`.
- Added examples under `examples/resources/ona_automation/` and `examples/data-sources/ona_workflows/`.
- Added direct import documentation to `examples/import.md`.
- Generated `docs/resources/automation.md` and `docs/data-sources/workflows.md`.
- Added `github.com/robfig/cron/v3` for backend-compatible cron validation and promoted `github.com/google/uuid` to a direct dependency.

## Difficulties encountered and resolutions

### Initial disabled state is not supported by CreateWorkflow

`CreateWorkflowRequest` has no `disabled` field. Creating an initially disabled workflow therefore requires a second API operation, which creates a risk of losing the remote workflow ID if that follow-up fails.

The resource writes the created workflow ID to Terraform state immediately, then sends a focused update when `disabled = true`. A hermetic acceptance test forces the follow-up update to fail and verifies that retry cleanup does not leave concurrent duplicate workflows.

### Executor omission and removal have different Terraform semantics

The API chooses the caller when no executor is provided during creation, but it has no valid operation for clearing an executor later. Sending an empty executor ID is rejected by the backend.

The executor is modeled as optional and computed. The API-resolved executor is stored after creation, removing the configuration block retains that observed executor, and changing it requires a valid user or service-account subject. Update ownership errors from the backend remain visible to users.

### API canonicalization could cause perpetual diffs

The API canonicalizes omitted descriptions and protobuf durations. For example, Terraform configuration may contain `60m` while the API returns `1h0m0s`.

The mapping layer preserves Terraform's null-versus-empty description intent and the configured duration spelling when it is semantically equal to the API value. A duration plan modifier also treats equivalent Go duration strings as equal.

### Existing workflows can contain unsupported fields

The first provider schema intentionally covers only the core workflow subset, while existing workflows may contain reports or workflow-level agent and Codex settings. Silently ignoring these fields would allow a later update to erase remote configuration.

Read and import inspect the complete remote workflow and return an actionable `Unsupported Ona Workflow` diagnostic when management would be lossy. Tests cover report actions, report steps, agent settings, Codex settings, and unreproducible legacy pull-request triggers.

### Workflow deletion can be asynchronous

The backend immediately removes idle workflows but marks workflows with active executions as deleting and completes cleanup asynchronously. Force deletion would cascade into executions, environments, and agent executions.

Terraform always sends `force = false`. A workflow reported with `spec.deleting = true` is treated as absent from managed state, and delete-not-found is treated as success.

### Pagination order is not stable enough for Terraform state

API page order should not determine Terraform state order, and an error on a later page must not publish earlier pages as partial success.

The data source accumulates every page before setting state and sorts the final results by workflow ID. The test server deliberately returns reverse-ID order and can fail after the first page, proving both provider-side sorting and all-or-nothing error handling.

### Terraform values may be unknown during planning

Nested list and set elements can be unknown during configuration validation. Decoding them directly into Go primitive slices can collapse unknown values or produce premature validation failures.

Validation now examines Terraform values element by element. Unknown values are deferred during configuration validation and rejected only if they remain unknown when an apply or data-source read requires concrete API input.

### Malformed API results needed defensive handling

A successful Get RPC containing no workflow previously looked indistinguishable from a not-found result, and a syntactically valid RFC 3339 year can still fall outside protobuf timestamp bounds.

An empty Get payload now produces a diagnostic instead of removing state. Failed-since timestamps are checked for both RFC 3339 syntax and protobuf validity. Regression tests cover both cases.

## Validation performed

The following checks pass:

```text
make fmt
make test-unit
TF_ACC=1 go test -v ./internal/provider -run 'TestAcc(Automation|Workflow)' -count=1
make lint
make build
make generate
git diff --check
```

Documentation generation was run repeatedly and verified to be idempotent. The focused acceptance suite is hermetic and covers lifecycle operations, import, no-op plans, updates, drift, remote deletion, executor retention and changes, description clearing, initial-disable failure recovery, graceful and not-found deletion, unsupported imports, API errors, pagination, sorting, and partial-page failure.

Credentialed live acceptance tests were not run because they require an Ona token and can create or delete real workflows. No credentials, webhook secrets, Terraform state, or generated provider override files were added to the worktree.

## Remaining delivery work

The implementation is ready for review. The remaining delivery steps are to inspect the final worktree, commit the intended files, open a pull request, and run any repository CI or explicitly authorized live acceptance validation.
