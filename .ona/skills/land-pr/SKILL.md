---
name: land-pr
description: Automate the full PR lifecycle from diff to merge-ready. Self-reviews code, opens a draft PR, triages bot review comments with the user, polls CI and fixes failures, resolves merge conflicts, and reports readiness. Use land-pr for the full lifecycle; use create-pr when you only need a PR opened without post-PR automation. Triggers on "land PR", "land this", "iterate on PR", "make CI green", "address reviews", "ship this", "create PR and iterate".
---

# Land PR

Full PR lifecycle from self-review through readiness report.

**Related skills:** Uses `review` for code quality analysis and `create-pr` for PR creation. Use `create-pr` alone when you only need a PR opened without post-PR automation.

## Phase 1: Self-Review

1. Run `review` in self-review mode on the current diff against main.
2. **User triage (the "triage pattern"):** Present findings via `ask_clarifying_questions`. Batch similar findings. For each batch, ask: fix now, skip, or discuss. Apply approved fixes. Skip the rest.

This triage pattern is reused in the automated review response (phase 3, step 3) — reference it as "triage pattern" rather than restating it.

## Phase 2: PR Creation

Run the `create-pr` skill. It handles cleanup, formatting, linting, branch creation, commit, push, changelog categorization, opening the draft PR, docs cross-reference, and post-PR summary.

No step in this phase blocks PR creation — if a step fails, `create-pr` continues to the next.

## Phase 3: Post-PR Lifecycle Loop

The unique value `land-pr` adds over `create-pr`. After the PR is open, iterate through review response, CI fixes, and conflict resolution until the PR is merge-ready.

### 3. Automated review response

After the PR is open, **wait for bot reviews to arrive** before triaging. Bots need time to analyze the PR.

**Waiting strategy:**
1. Wait 1 minute after PR creation before the first check.
2. If no comments yet, poll every 30 seconds.
3. Stop waiting when comments appear OR after 3 minutes total (whichever comes first).

```bash
gh api /repos/{owner}/{repo}/pulls/{pr}/comments  # inline review comments
gh api /repos/{owner}/{repo}/issues/{pr}/comments  # general PR comments
gh api /repos/{owner}/{repo}/pulls/{pr}/reviews    # review submissions
```

Distinguish bot comments from human comments by checking the author's `type` field (`Bot` vs `User`).

Apply the triage pattern (phase 1, step 2) to bot findings. For each approved fix:
1. Make the code change.
2. Reply to the comment explaining the change.
3. Resolve the thread via GraphQL:
   ```graphql
   mutation { resolveReviewThread(input: { threadId: "<thread-node-id>" }) { thread { isResolved } } }
   ```

For skipped findings, reply with reasoning and leave the thread open.

### 4. CI polling and fix loop

Wait for CI to start before polling. Then check status periodically until all checks complete.

```
github_pull_request_read(get_check_runs) → list CI jobs and their status
```

**On success:** All checks pass → move to step 5.

**On failure:**
1. Identify the failed job and read its logs:
   ```
   github_workflow_run_read(get_jobs, run_id: <workflow_run_id>)
   github_workflow_run_read(get_job_log, job_id: <failed_job_id>, tail: true)
   ```
2. Diagnose and fix. Commit and push.
3. Return to polling.

**Max 3 CI fix iterations.** After 3 failed attempts, report to the user what failed, what was tried, and what's still broken. Ask whether to continue or hand off. The counter resets when re-entering the CI loop from a different step.

### 5. Merge conflict resolution

Check for merge conflicts before reporting completion.

```bash
git fetch origin main
git merge origin/main --no-commit --no-ff 2>&1  # dry-run merge
```

If no conflicts → move to step 6.

If conflicts exist:
1. Analyze changes on both sides to understand intent.
2. Resolve conflicts preserving functionality from both branches.
3. Run `gofmt` on modified Go files.
4. Run related tests to verify the resolution.
5. If tests pass → commit the merge and push, then return to step 4 (CI must pass again).
6. If tests fail → abort the merge, show the user which files conflict and what both sides changed, ask how to resolve.

### 6. Completion

The loop ends when ALL of these are true:
- CI is green on the latest commit.
- No unresolved automated review comments remain.
- No merge conflicts with main.

Report readiness:

```
PR is ready for merge.

- CI: all checks passing
- Reviews: all automated comments resolved
- Conflicts: none
- PR: <link>
```

Do not auto-merge. The user merges manually.

**Early termination:** The loop also ends if the 3-iteration CI limit is hit, the agent has questions it can't answer, or CI hasn't completed after a reasonable wait. In all cases, report the current state and what's blocking.

## Todo Labels

Use short, action-oriented labels. Ordering conveys the flow.

```
Self-review diff
User triage
Create PR
Address review comments
CI polling and fixes
Merge conflict check
```

## Anti-patterns

- Don't re-specify `create-pr` or `review` steps — call those skills.
- Don't amend pushed commits to fix CI — create new commits.
- Don't resolve review threads without replying first.
- Don't auto-fix subjective feedback — surface to user.
- Don't auto-merge — report readiness and let user merge.
- Don't poll indefinitely — report and pause if CI stalls.
- Don't check for review comments immediately after PR creation — bots need time to analyze. Always wait before the first check.
