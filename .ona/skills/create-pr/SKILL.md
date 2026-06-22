---
name: create-pr
description: Create a pull request for code changes. Use when asked to create a PR, open a PR, submit changes, or prepare changes for review.
---

# Create PR

Open a reviewable pull request from the current repository changes.

## Tool Preference

Prefer SCM-integrated tools for repository-hosted operations whenever an equivalent tool exists: opening or updating PRs, applying labels, posting comments, editing descriptions or titles, and searching prior PRs. Use local `git` commands for local repository operations such as branching, staging, committing, and pushing.

If no SCM-integrated tool is available, use the project wrapper or native SCM CLI as a fallback and mention the fallback briefly in the summary.

## Workflow

1. Inspect the working tree and identify unrelated changes.
2. Run the project-appropriate formatting and verification commands for the changed files.
3. Create a feature branch from the current base branch unless the user has already chosen one.
4. Stage only the intended files.
5. Commit with a concise Conventional Commit message when that fits the project.
6. Push the branch to the remote.
7. Open a draft PR using the repository's pull request template when one exists.
8. Summarize the branch, commit, PR link, verification run, and any skipped checks.

## Branch Naming

Use a short branch name that describes the change:

```text
<initials>/<short-description>
```

If user initials are not available, use a neutral prefix such as `dev/`.

## PR Description

Keep the PR description focused on user-visible intent and verification:

```markdown
## Summary
- What changed
- Why it changed

## Verification
- Commands run
- Any checks not run, with reason
```

Avoid implementation trivia unless it affects review, migration, operation, or rollback.

## Special Cases

- If generated files changed, explain how they were generated.
- If migrations changed, do not edit existing committed migrations; create a new migration instead.
- If documentation is needed but not included, call that out in the PR summary.
- If analytics or telemetry events changed, include a short event-change report using `references/analytics-event-changes-report-template.md`.

## Anti-Patterns

- Do not push directly to the default branch without explicit user approval.
- Do not commit unrelated changes.
- Do not overwrite or revert user changes unless explicitly requested.
- Do not amend pushed commits just to fix follow-up issues; create new commits instead.
- Do not auto-merge the PR.
- Do not invent labels or release-note sections unless the repository already uses them.
