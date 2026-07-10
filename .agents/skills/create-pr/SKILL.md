---
name: create-pr
description: Create a pull request for changes in terraform-provider-ona. Use when asked to create a PR, open a PR, submit changes, land changes, prepare a branch, commit changes, write a PR description, or summarize verification.
---

# Create PR

Prepare a focused reviewable PR that preserves unrelated user work.

## Tool Preference

Prefer SCM-integrated tools for repository-hosted operations whenever an equivalent tool exists: opening or updating PRs, applying labels, posting comments, editing descriptions or titles, and searching prior PRs. Use the GitHub CLI or direct GitHub API calls only as a fallback when no SCM tool is available for the needed operation, and mention the fallback briefly in the summary.

Use local `git` commands for local repository operations such as branching, staging, committing, and pushing.

## Steps

1. **Inspect state** - run `git status --short --untracked-files=all` and `git diff --stat`. Preserve unrelated user work.
2. **Clean up code** - remove obvious temporary comments, debug output, and accidental local-only files from the requested change.
3. **Format** - run the format make target, `make fmt`, which covers Go and Terraform formatting.
4. **Lint** - run `make lint` for Go/provider changes. For guidance-only changes, run Markdown/frontmatter checks and the skill audit when available.
5. **Generate** - if provider schemas, docs, or examples changed, run `make generate` and then `git diff --exit-code` unless generated files are expected and included.
6. **Test/build** - run `make test` and `make build` for provider code changes. The test target runs both unit and acceptance tests.
7. **Identify Linear issue** - see [Linear Issue Detection](#linear-issue-detection). Never blocks.
8. **Create branch** - `<initials>/<short-description>`; keep it short, preferably 24 characters or fewer, and get initials from `git config user.name`.
9. **Commit** - use [Conventional Commits](https://www.conventionalcommits.org/). Do not add AI co-author trailers.
10. **Push** - `git push -u origin <branch-name>`.
11. **Changelog categorization** - auto-skip or best-guess. See [Changelog Categorization](#changelog-categorization). Never blocks.
12. **Open draft PR** - use an SCM PR creation tool and `.github/pull_request_template.md`. Apply the changelog label chosen in step 11 if labels are available in this repository.
13. **Docs check** - check if provider behavior changes need updates to `README.md`, `docs/**`, `examples/**`, or `dev/local-devloop/README.md`. Use the `technical-writing` skill for substantial docs work.
14. **Post-PR summary** - report the branch name, PR link, Linear issue status, changelog label/status, docs status, and verification commands actually run.

## Linear Issue Detection

Never ask the user. Proceed silently and report the outcome in the summary.

1. If the full Linear issue ID is already in context, use it directly.
2. Extract a numeric issue hint from the branch name when present, such as `1234` from `ab/1234-provider-docs`.
3. If Linear tools are available, resolve the full ID by checking active teams and trying `TEAM-<numeric>` until found.
4. If found, use it. If multiple candidates match, pick the best match from title/context.
5. If not found, proceed without a Linear issue association.
6. Report either `Linked to TEAM-1234` or `No Linear issue detected`.

## Changelog Categorization

Before opening the PR, determine a changelog category. Never block PR creation on changelog categorization; if labels are unavailable, report the selected category without applying a label.

### Workflow

1. **Auto-skip rules.** Use `changelog:skip` if any of these are true:
   - Commit type is `chore`, `ci`, `build`, `test`, `style`, or docs-only.
   - Title starts with `Revert`.
   - The change is internal-only guidance, generated cleanup, dependency maintenance, release plumbing, or CI-only work.
   - The feature is incomplete, gated, experimental, or part of a larger unshipped sequence.

   When auto-skipping, mention it in the summary: `Changelog: changelog:skip - <reason>.`

2. **Best-guess label.** If no auto-skip rule matched, pick a label based on commit type:

   | Commit type | Best-guess label |
   |---|---|
   | `feat` | `changelog:feature` |
   | `fix` | `changelog:fix` |
   | `refactor`, `perf` | `changelog:improvement` |
   | anything else | `changelog:skip` |

   When best-guessing, mention it in the summary: `Changelog: <label> (best guess from commit type).`

3. **Apply the label** only if the label exists or an SCM label tool can create it safely. If label operations fail, keep the PR moving and report the failure.

### Changelog label to commit type mapping

Use this mapping if the user asks to change the changelog category:

| Changelog label | Commit type |
|---|---|
| `changelog:feature` | `feat` |
| `changelog:improvement` | `refactor` or `perf` |
| `changelog:fix` | `fix` |
| `changelog:skip` | keep existing type unless the user asks otherwise |

Do not amend pushed commits or force-push unless the user explicitly approves it.

## PR Description

Use `.github/pull_request_template.md`:

```markdown
## Description

## Related Issue(s)
Fixes <issue-ref>

## How to test
```

Keep the description brief and concrete. Focus on business value and the change by top-level folder; do not use fluffy language, implementation details, or individual commit summaries.

## Docs Check

After the code changes are ready and before opening the PR:

1. Decide whether changed behavior affects Terraform users, provider docs, examples, release instructions, or local dev-loop docs.
2. If no docs changes are needed, set the PR description's testing or description text to say `No docs changes needed.`
3. If docs changes are needed, update them in the same branch unless the user explicitly asks for a separate docs PR.
4. Run `make fmt` for formatting and `make generate` when generated provider docs should change.
5. If docs validation fails, do not block PR creation indefinitely. Report the failure and the command that failed.

## Verification Text

Report only commands actually run.

For schema, docs, or examples changes, explicitly state whether `make generate` was run and whether generated output was clean.

## Special Cases

- **Guidance-only changes:** run frontmatter/Markdown checks and the skill audit when available; do not run provider build/test commands unless the guidance change affects checked automation.
- **Docs-only changes:** run `make fmt` and `make generate` if generated provider docs are affected.
- **Release changes:** check `docs/release.md`, `scripts/**`, and `.github/workflows/**` together.
- **Tests:** run `make test`.

## Anti-patterns

- Do not use superlatives in commits or PR descriptions.
- Do not include unnecessary implementation detail in PR descriptions.
- Do not commit unrelated changes.
- Do not push directly to `main`.
- Do not amend pushed commits or force-push unless explicitly approved.
- Do not claim test coverage unless the relevant command ran.
- Do not overwrite the PR body without first reading the current template or existing description.
- Do not let docs generation failure block opening a code PR forever; report the failure and continue when the code PR is otherwise ready.

## Done Criteria

PR preparation is done when unrelated work is preserved, formatting/lint/test/generation checks appropriate to the diff have run or been explicitly skipped, changelog and Linear status are reported, docs impact is handled, and the PR description uses the repository template.

## When Stuck

- If there are unrelated local changes, leave them unstaged and mention them.
- If verification cannot run because a tool or credential is missing, report the exact blocker and skipped command.
- If SCM tooling is unavailable, prepare the branch, commit, and PR text locally and explain what remains.
