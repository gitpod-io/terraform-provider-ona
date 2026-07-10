---
name: review
description: Review code changes in this Terraform provider repository. Use when asked to review, self-review, check a diff, review a PR, or assess changes to Go provider code, Terraform docs, examples, generation, release scripts, or CI for terraform-provider-ona.
---

# Review

Review for bugs, regressions, missing tests, and provider-state mistakes. Suppress nitpicks unless they hide a real risk.

## Modes

### Self-review mode

Review the local diff against the merge-base with `main`:

```bash
git fetch origin main
BASE=$(git merge-base origin/main HEAD)
git diff "$BASE" --name-only
git diff "$BASE"
```

Diff against the merge-base, not `origin/main` directly, so unrelated changes on `main` do not appear as phantom removals.

### PR review mode

Read the PR description, changed files, and diff using the available SCM tools. Post inline comments only for concrete findings tied to changed lines.

## Workflow

1. Detect scope:
   - Provider code: `internal/provider/**`.
   - API wrapper code: `internal/client/**`.
   - Documentation generation: `tools/**`, `GNUmakefile`, generated `docs/**`.
   - Copied/generated API subset: `internal/api/go/**`.
   - Docs/examples: `docs/**`, `examples/**`, `README.md`, `dev/local-devloop/README.md`.
   - Import/release tooling: `scripts/**`, `docs/release.md`.
2. Gather intent from the user request, PR description, issue references, or commit messages.
3. Read enough surrounding code to understand existing patterns before judging the diff.
4. Analyze provider behavior first, then tests/docs/generated output.
5. Output findings first, ordered by severity, with file and line references.

## Checks

- Terraform lifecycle: Create, Read, Update, Delete, ImportState, and data source reads refresh state truthfully.
- State model: unknown, null, computed, optional, required, sensitive, and write-only values are handled without zero-value collapse.
- Drift and planning: stable computed fields use appropriate plan modifiers, immutable fields require replacement, unordered remote values do not produce perpetual diffs.
- Secrets: sensitive values are not logged, persisted, or exposed unless Terraform state storage is intentional and documented.
- Diagnostics: API and validation failures produce actionable diagnostics rather than panics or generic errors.
- Tests: provider behavior has targeted tests; importable resources cover import; generator changes cover output shape; scripts cover failure cases.
- Generated output: schema, docs, and examples changes include `make generate` output and a clean `git diff --exit-code` when relevant.
- Boundaries: do not request hand edits to `internal/api/go/**` for style-only issues; copied API changes must come from an intentional sync.
- Docs/examples: user-facing behavior changes update examples or docs when users need different Terraform configuration.

## Output

Classify findings by severity:

- **P0 must be done:** something is broken, unsafe, regresses behavior, or does not meet the stated acceptance criteria.
- **P1 should be done:** an optimization or improvement that makes the code easier to read, easier to maintain, or faster, but does not block correctness.
- **P2 could be done:** style or idiom feedback where the code is functional, performant enough, and not buggy, but does not match this repository or the language/library/framework context.

Write each finding's comment text as a [Conventional Comment](https://conventionalcomments.org/): `<label> [decorations]: <subject>`. Keep severity and file location as metadata before the comment, not inside the Conventional Comment itself. Add a short follow-up sentence only when the impact or fix is not obvious from the subject.

- P0 uses `issue (blocking)` and must say the fix **must** be done.
- P1 uses `suggestion (should)` and must say the improvement **should** be done.
- P2 uses `nitpick (could)` and must say the style or idiom adjustment **could** be done.

Use this format:

```markdown
Findings:
- [P0] file:line
  issue (blocking): <fix> must be done because <behavior is broken or acceptance criteria are unmet>.
- [P1] file:line
  suggestion (should): <improvement> should be done to improve <readability, maintainability, or performance>.
- [P2] file:line
  nitpick (could): <style or idiom adjustment> could be done to match <repo, language, library, or framework convention>.

Open questions:
- ...

Summary:
- ...
```

If there are no findings, say so clearly and mention any residual test or acceptance-test gap.

## Done Criteria

A review is done when the changed files have been checked against the relevant provider, docs, generation, or tooling risks; findings are severity-ranked with file/line references; and any skipped generated or credentialed areas are named.

## When Stuck

- If intent is unclear, review the code on its own merits and state the missing context.
- If the diff is very large, prioritize P0/P1 provider-state, data-loss, secret-leak, and CI-blocking issues.
- If generated files dominate the diff, verify the generator input and output consistency instead of reviewing generated lines one by one.
