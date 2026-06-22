---
name: review
description: Code quality analysis for Go backend and React/TypeScript frontend changes. Supports self-review before opening a PR and PR review after a PR exists. Suppresses nitpicks and focuses on bugs, regressions, missing tests, security, maintainability, and user-facing quality.
---

# Review

Code quality analysis for backend and frontend changes. Auto-detect the relevant checks from the changed file extensions and repository layout.

**Related skills:** `land-pr` uses this skill for pre-PR self-review.

## Modes

### Self-Review Mode

Review the local diff against the merge-base with the default branch. Output findings to the user grouped by severity.

```bash
git fetch origin
BASE=$(git merge-base origin/HEAD HEAD)
git diff "$BASE" --name-only
git diff "$BASE"
```

Always diff against the merge-base. Comparing directly against the latest default branch tip can include unrelated changes that landed after the current branch diverged.

### PR Review Mode

Review a pull request diff. Use available SCM-integrated tools to read changed files and diff hunks. If an SCM tool is unavailable, use the repository's native CLI or web UI as a fallback.

## Workflow

### 1. Detect Scope

Classify changed files:

- **Backend:** `.go`, `.sql`, migration, or service-side files -> load [references/backend.md](references/backend.md)
- **Frontend:** `.tsx`, `.jsx`, `.ts`, `.js`, `.css`, or UI files -> load [references/frontend.md](references/frontend.md)
- **Mixed:** load both references

Skip generated files such as `*.pb.go`, `*.gen.ts`, and vendored files. Mention skipped generated files in the review summary.

### 2. Gather Context

- Read the PR description, issue reference, or commit messages for stated intent.
- For each changed function/type, trace nearby callers when needed to assess regression risk.
- Read relevant tests and fixtures before deciding whether coverage is sufficient.

### 3. Analyze

Run the relevant checks from the loaded references. Classify each finding:

- **[P0] Must fix:** Bugs, security flaws, regressions, data loss, or missing error handling on critical paths.
- **[P1] Should fix:** Maintainability issues, missing tests for new behavior, architecture drift, or user-facing quality problems.
- **[P2] Suggestion:** Polish that improves quality but should not block progress.

Suppress nitpicks: style preferences, minor naming, import ordering, or formatting that tooling already handles.

### 4. Output Findings

For self-review, present findings grouped by severity. Each finding includes:

1. What's wrong.
2. What to do.
3. Why it matters.

For PR review, post a concise summary and inline comments for concrete [P0] and [P1] findings when tool support is available.

```markdown
## Code Review

### [P0] Must Fix
[findings]

### [P1] Should Fix
[findings]

### [P2] Suggestions
[findings]

### Notes
[important context, skipped files, or test gaps]
```

## Intent Alignment

After analyzing code quality, check whether the diff matches the stated goal:

- Flag unrelated files or behavior changes as scope creep.
- Flag missing documentation when user-facing behavior changes.
- Flag missing migration or compatibility notes when data shape, API shape, or operational behavior changes.

## Definition of Done

A review is complete when:

1. Relevant changed files have been analyzed against the appropriate checklist.
2. Findings are categorized as [P0], [P1], or [P2].
3. Output matches the active mode.
4. Intent alignment has been checked.

## When Stuck

- If intent is unclear, review the code on its own merits and say intent was unclear.
- If the diff is too large for a thorough review, focus on [P0] findings and state the limit.
- If generated files appear, skip them and review the generator or source inputs instead.
