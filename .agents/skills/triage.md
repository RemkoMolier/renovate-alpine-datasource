# Triage

The procedure for working through issues labeled `triage` in this repository. Also covers decomposition (splitting an oversized issue into sub-issues), since that's a triage decision. See [AGENTS.md](../../AGENTS.md#issue-triage) for the policy; this file is the operational how-to.

## List the inbox

```sh
gh issue list --label triage --state open
```

## Triage one issue

Read the issue. Identify its template (the auto-applied label tells you: `agent-task`, `bug`, or `enhancement`) and follow the matching procedure below.

Every triage action ends with **removing the `triage` label in the same `gh issue edit` call** that applies the routing decision. One edit, one state transition.

### agent-task

0. **Check if it's a parent (has sub-issues).** Query `subIssuesSummary`:
   ```sh
   gh api graphql -f query='{ repository(owner:"'$GH_OWNER'",name:"'$GH_REPO'") { issue(number:'$N') { subIssuesSummary { total completed } } } }' \
     --jq '.data.repository.issue.subIssuesSummary'
   ```
   If `total > 0`, this is a parent of a decomposed task. **Do not dispatch it.** Verify the decomposition looks reasonable (children exist, dependencies are declared where needed), then `gh issue edit <n> --remove-label triage` and move on. The children will be triaged independently.

1. Verify the spec is complete. All eight sections must be filled:
   - Context (one paragraph of motivation)
   - Problem (a precise statement, not a solution)
   - Acceptance Criteria (at least one testable item; the placeholder `- [ ]` lines are not acceptable)
   - Definition of Done (default checklist; any `~~strikethrough~~` must have a rationale)
   - Files in scope (concrete paths or directories)
   - Files out of scope
   - Verification (concrete commands, not prose)
   - Notes & hints (optional)
2. If incomplete: post a comment listing exactly which pieces are missing, leave `triage` on, move to the next issue.
3. **Check if it should be split.** Apply the criteria from [When to decompose](#when-to-decompose). If any fire, jump to [Decomposition](#decomposition) — the parent is decomposed into sub-issues and the children re-enter triage as independent agent-tasks.
4. **Check `blocked by` dependencies.** Query the issue's blockers; if any are open, skip — the child isn't ready:
   ```sh
   gh api graphql -f query='{ repository(owner:"'$GH_OWNER'",name:"'$GH_REPO'") { issue(number:'$N') { blockedBy(first:50) { nodes { number state } } } } }' \
     --jq '.data.repository.issue.blockedBy.nodes[] | select(.state=="OPEN") | .number'
   ```
   If the command prints any issue numbers, leave `triage` on; the child will be re-triaged after its blockers merge.
5. If complete, reasonably sized, and unblocked: pick a routing using the [Routing heuristics](#routing-heuristics) below, then apply in one edit:
   - **Copilot**: `gh issue edit <n> --add-assignee "copilot-swe-agent[bot]" --remove-label triage`
   - **OpenHands**: `gh issue edit <n> --add-label openhands --remove-label triage`
   - **Maintainer**: `gh issue edit <n> --remove-label triage` (no routing label applied)

### bug

1. Try to reproduce from the steps in the issue.
2. If you cannot reproduce, post a comment asking for the missing information (config, version, exact command). Leave `triage` on.
3. If you can reproduce:
   - Add area / priority labels if useful (create them on first use).
   - If the fix is small and well-scoped, **convert to an agent-task**: file a new issue using the agent-task template with a full spec, cross-link both directions (`References: #<bug>` in the spec; `Spec: #<agent-task>` in the bug). The original bug stays open as the user-facing tracking issue.
4. Remove `triage` from the bug.

### enhancement

1. Discuss / refine in the issue thread until the proposal is concrete enough to spec.
2. If accepted, convert to an agent-task as above.
3. If declined, close the issue with a one-paragraph rationale.
4. Remove `triage`.

## Routing heuristics

When an agent-task spec is complete, choose where it goes:

**Copilot** is best for:
- Single-file or single-package changes.
- Clear bug fixes that already have a reproducer.
- Mechanical additions: a new struct field, a flag, a CLI subcommand, an exported helper.
- Test-only additions where the production code is unchanged.

**OpenHands** is best for:
- Multi-file changes with a clear written spec.
- Adding a new package or subsystem.
- Migration-style changes (renaming, restructuring) with a clear before/after.
- Tasks that benefit from filesystem exploration before editing.

**Maintainer (no routing label)** for:
- Anything that touches `.github/workflows/`, `.github/dependabot.yml`, `go.mod`, `.golangci.yml`, or release tooling.
- Security-sensitive changes (auth, crypto, secrets handling, input parsing on untrusted data).
- First-pass design work when the spec is honestly uncertain — better to refine the spec than dispatch a stab in the dark.
- Anything you would not want a stranger to autonomously implement.

When in doubt between two options, prefer the more conservative one (maintainer > OpenHands > Copilot for autonomy; the reverse for cost).

## Decomposition

Splitting a too-big agent-task issue into sub-issues using GitHub's native sub-issues (GA April 2025) and `blocked by` dependencies (GA August 2025).

### When to decompose

Decompose when any of these fire — LOC is the last and weakest signal:

- **More than one concern.** "Add parser AND add HTTP handler AND wire response" is three concerns. If you can't summarise the work without an "and", split.
- **Acceptance Criteria cluster into independent groups.** Each cluster can ship on its own → each cluster is its own child issue.
- **Horizontal layering would be needed.** If the natural shape is "build data model, then API, then wiring," prefer instead vertical slices that each ship a thin end-to-end change. Each slice exercises the full stack and produces no dead code while waiting for siblings.
- **Reviewing in one sitting (5–15 minutes) feels implausible.** Even at low LOC, dense concurrency or subtle refactors can blow past a one-sitting review.
- **The smallest reasonable slice still exceeds ~200 LOC** *and* the change isn't a single cohesive thing (cohesive: a vendored library update, a generated table, a struct-of-fields). LOC alone never forces a split; it just triggers a re-audit against the criteria above.

If you're under 200 LOC but the issue feels sprawling: still decompose. Small PRs review faster and are safer to revert. LOC is a smell, not a gate.

### At triage (canonical case)

1. **Do not dispatch the parent.** Leave it unrouted; the parent is now a hierarchy root.
2. Sketch the child slices (one per ~200 LOC PR). Identify any **dependency order** between them.
3. For each slice, file a child issue using the agent-task template:
   ```sh
   gh issue create --template agent_task.yml \
     --title "<slice title>" \
     --body "..."
   ```
4. **Attach each child as a sub-issue of the parent** via GraphQL:
   ```sh
   PARENT_ID=$(gh api graphql -f query='{ repository(owner:"'$GH_OWNER'",name:"'$GH_REPO'") { issue(number:'$PARENT') { id } } }' --jq .data.repository.issue.id)
   CHILD_ID=$(gh api graphql -f query='{ repository(owner:"'$GH_OWNER'",name:"'$GH_REPO'") { issue(number:'$CHILD') { id } } }' --jq .data.repository.issue.id)
   gh api graphql -f query='mutation { addSubIssue(input: {issueId:"'$PARENT_ID'", subIssueId:"'$CHILD_ID'"}) { issue { number } } }'
   ```
   GitHub renders the parent with a progress bar over its sub-issues automatically.
5. **Declare ordering as `blocked by` dependencies** for children that have prerequisites:
   ```sh
   B_ID=$(gh api graphql -f query='{ repository(owner:"'$GH_OWNER'",name:"'$GH_REPO'") { issue(number:'$B') { id } } }' --jq .data.repository.issue.id)
   A_ID=$(gh api graphql -f query='{ repository(owner:"'$GH_OWNER'",name:"'$GH_REPO'") { issue(number:'$A') { id } } }' --jq .data.repository.issue.id)
   gh api graphql -f query='mutation { addBlockedBy(input: {issueId:"'$B_ID'", blockingIssueId:"'$A_ID'"}) { issue { number } } }'
   ```
6. Remove `triage` from the parent. The presence of sub-issues itself signals "this is a parent" — no separate label needed (triage's step 0 detects parents via `subIssuesSummary.total > 0`).
7. Each child re-enters the inbox as a normal agent-task issue. Triage will skip blocked children until their blockers merge.

Children are routed independently per the [Routing heuristics](#routing-heuristics) above. It is normal for different sub-issues of the same parent to go to different agents.

### Mid-implementation (edge case)

If you are partway through a PR and realize it won't fit:

1. **Stop coding.** Do not push past the cap with a "flag in PR body" note.
2. Identify the smallest coherent slice of what you've already done that can stand on its own — passing tests, no dangling references, no half-built APIs.
3. **Scope the current PR down** to that slice.
4. File follow-up child issues for the remaining work (`gh issue create ...`) and attach them as sub-issues of the parent (step 4 above). Declare blocked-by dependencies where they exist (step 5).
5. Remove any agent routing from the parent (un-assign, drop labels) — the parent is no longer an implementation task.
6. Continue the current PR against the smaller scope. The next unblocked sub-issue can be triaged and dispatched in parallel.

The reviewer should see: one coherent small PR, a parent issue with a clear sub-issue plan, and child issues queued for triage.

### When NOT to decompose

- A truly atomic change that genuinely exceeds 200 LOC because of unavoidable boilerplate (large generated table, repetitive struct definitions). Flag in the PR body with rationale, ship it whole.
- A migration that must land atomically to avoid an intermediate broken state. Same — flag and ship whole.

In both cases, prefer to discuss in a comment on the issue before opening the oversized PR.

## When to skip an issue

- Already has a routing assignment or the `openhands` label → already dispatched. Just remove `triage` if it's still there.
- Filed by a bot (Dependabot, Renovate, etc.) → those flow through their own mechanisms; remove `triage`, don't add routing.
- Closed → ignore.
