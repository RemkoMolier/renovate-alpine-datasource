# AGENTS.md

Authoritative rules for any contribution to this repository, whether by a human or an AI agent. Read this file first. If anything in this file conflicts with a request, follow this file.

## Project

`renovate-alpine-datasource` is a Go service that exposes Alpine Linux package metadata in a shape consumable by [Renovate's `customDatasources`](https://docs.renovatebot.com/modules/datasource/custom/) mechanism. The repository ships a single binary; there is no library API today.

## Build, test, lint

All checks are wrapped in the [`Makefile`](Makefile) — no separate tool install needed (lint and coverage run via pinned `go run` invocations and are cached after the first call):

```sh
make verify   # build + vet + test + lint + coverage gate (run before opening any PR)
make help     # list every available target
```

Individual targets if you need to iterate: `make build`, `make vet`, `make test`, `make lint`, `make coverage`, `make fmt`, `make tidy`.

`make verify` must be green locally before a change is considered done. CI runs the same set across a Go matrix (latest minor and the one before), plus a Conventional Commits PR-title check. **PRs do not merge with red CI** — see [PR conventions](#pr-conventions).

## Repo map

- `main.go` — entrypoint stub.
- `go.mod` — module declaration; floor pinned to "latest minor minus one" (see [Go version policy](#go-version-policy)).
- `.golangci.yml` — lint config. Enables the analyzers `gopls` runs (`standard` set + `unparam`, `usestdlibvars`, `intrange`, `copyloopvar`, all `govet` analyzers except `fieldalignment`).
- `.github/workflows/` — `ci.yml` (build/test/lint matrix) and `pr-title.yml` (Conventional Commits enforcement).
- `.github/dependabot.yml` — weekly grouped updates for `gomod` and `github-actions`.
- `.github/ISSUE_TEMPLATE/` — issue forms (bug, feature request, agent task).
- `.github/copilot-instructions.md` — thin pointer to this file for GitHub Copilot.
- `.agents/skills/` — procedural how-tos, agent-readable. See [Skills](#skills).
- `CONTRIBUTING.md` — human-facing setup and contribution flow.

## Coding conventions

### Go version policy

Support the latest released Go minor version and the one before (Go's own support window). When a new minor releases, the floor in `go.mod` moves up. Do not add backward-compat shims for older versions.

### Idiomatic Go

Follow Effective Go and the Uber Go Style Guide; for project-specific rules and the most common pitfalls in this repo, see [`.agents/skills/idiomatic-go.md`](.agents/skills/idiomatic-go.md). The short version:

- Use `any`, never `interface{}`.
- Wrap errors with `fmt.Errorf("...: %w", err)`. Compare with `errors.Is` / `errors.As`. Avoid `==` on errors.
- Tests are table-driven with `t.Run` subtests. Prefer `t.Cleanup` over `defer` for test teardown.
- `context.Context` is the first argument for any function doing I/O.
- Do not `panic` outside `main` / `init`.
- Lowercase, no-underscore package names matching the import path's final segment.
- Prefer small interfaces defined at the consumer.

### Scope discipline

Prefer minimal, surgical changes over broad refactors. If you find yourself fixing tangentially related code while addressing an issue, stop — open a separate issue for the tangent. **Unrelated changes in one PR are a sign the scope was wrong, not a feature.**

## PR conventions

- **Conventional Commits in the PR title.** The PR title becomes the squash-merge commit subject and is validated by the `pr-title` workflow. Common prefixes: `feat`, `fix`, `docs`, `chore`, `ci`, `refactor`, `test`, `perf`. Add a scope when it helps (`feat(apk): ...`).
- **Commit messages and PR bodies are about *why*, not *what*.** The diff already shows what changed. The PR body's `## Why` section (which becomes the squash-merge commit body) must explain the motivation — what the change enables, what bug it prevents, what tradeoff was accepted — so the future reader doing `git blame` 18 months from now understands the *reason* the code looks the way it does.
- **TDD is the norm for behavior-changing work.** Write a failing test first, watch it fail, then write the minimum code to make it pass. The **coverage gate** (CI; thresholds in `.testcoverage.yml`) mechanically enforces that new packages have tests — patch-coverage-like in a greenfield repo, since a new package without tests starts at 0% and fails the 80% package threshold. Exceptions (refactor / docs / build / dep-bump / test-only) are called out in the PR's `## Why` section. Procedure in [`.agents/skills/tdd.md`](.agents/skills/tdd.md).
- **CI green is a merge prerequisite.** A PR does not merge while any of the `Test (Go 1.25)`, `Test (Go 1.26)`, `Lint`, `Coverage`, or `PR title / Validate Conventional Commits format` checks are red. Branch protection on `main` enforces this — see the [branch protection note](#branch-protection-on-main) below.
- **SHA-pin every new GitHub Action.** Format: `org/repo@<full-commit-sha> # vX.Y.Z`. The trailing version comment is what Renovate / Dependabot use to surface updates. See [Adding a GitHub Action](#recipe-adding-a-github-action) for the exact procedure.
- **AI-assistant attribution.** For commits *you* author (with AI assistance from Claude, Cursor, IDE Copilot completion, etc.), do not add `Co-Authored-By:` lines for the AI — the human is the sole author. Bot-authored PRs from agents that produce their own commits (Copilot's coding agent, OpenHands Cloud) are different: those commits' `Author` is the bot, with the human assigner appended as `Co-authored-by:`. That pattern is allowed and preserves the audit trail of which agent did the work.
- **Reviewer scope.** Reviewers only leave feedback — comments, suggestion blocks, and an approve / request-changes / comment verdict. They do **not** edit the PR description, push fixup commits to the author's branch, or otherwise amend the author's work. Every change to the PR goes through the author (or the dispatched agent), even typos. The agent or human author owns the PR end-to-end. See [`.agents/skills/review.md`](.agents/skills/review.md).
- **Squash-merge commit body comes from the PR's `## Why` section.** The repo is configured with `squash_merge_commit_title = PR_TITLE` and `squash_merge_commit_message = PR_BODY`, so the merged commit on `main` is `<conventional commit title>` + the PR body. The alternative (`COMMIT_MESSAGES`) concatenates every branch commit including bot placeholder commits, debug trailers, and duplicate `Co-authored-by` lines — not the history we want. For bot-authored PRs (Copilot, OpenHands), the squash-merge commit's `Author` becomes the bot identity (e.g. `copilot-swe-agent[bot]` or `openhands <openhands@all-hands.dev>`), not the maintainer who merged it. This is expected and preserves the audit trail: the bot did the work, the human reviewed and merged.
- **PRs are one concern, reviewable in one sitting, independently revertable, with self-contained tests.** Prefer vertical slices (a thin end-to-end change) over horizontal layers (build the data model first, then the API, then the wiring) — vertical slices ship value and get exercised end-to-end; horizontal layers batch risk. PRs over ~200 LOC of diff are a warning sign to audit against these criteria; flag in the description with rationale if you decided the overrun was justified (e.g. cohesive boilerplate, vendored library update).
- **Squash-merge is the default.** Branches are short-lived; `main` is always the integration point.
- **Never bypass hooks.** No `--no-verify`, no `--no-gpg-sign`. If a hook fails, fix the underlying problem.
- **Never force-push to `main`.** Force-push to your own topic branch is fine.

### Branch protection on `main`

`main` is branch-protected. The following checks are required to pass before merge:

- `Test (Go 1.25)`
- `Test (Go 1.26)`
- `Lint`
- `Coverage`
- `Validate Conventional Commits format`

Direct pushes to `main` are blocked; every change lands via PR. Force-pushes to `main` are blocked unconditionally. If you need to update branch protection rules, edit them via `gh api repos/$GH_OWNER/$GH_REPO/branches/main/protection` and document the change in a PR that updates this section.

### Repo settings beyond branch protection

- **Workflow approval for first-time contributors.** The repo's Actions → *Fork pull request workflows from outside collaborators* setting is `first_time_contributors_new_to_github` (not the stricter `first_time_contributors`). This means Copilot's bot pushes can still trigger `action_required` on the first commit of a new PR, in which case a one-time manual `gh run rerun <id>` is still required; subsequent pushes on that PR auto-run, so the looser setting avoids needing a manual rerun on every amend. For archaeology, the corresponding API write is `gh api -X PUT repos/$GH_OWNER/$GH_REPO/actions/permissions/fork-pr-contributor-approval -f approval_policy=first_time_contributors_new_to_github`. If the setting ever reverts (e.g. repo re-creation), re-apply it via the repo Settings UI or that API endpoint.

## Don't-touch zones

- `.github/dependabot.yml` — do not modify without discussion in an issue first; this controls the cadence of every other dependency change.
- `LICENSE` — never edited as part of an unrelated change.
- Anything outside the **Files in scope** list of the issue you are implementing. If you discover you need to, post a comment on the issue and wait.

## Issue triage

Every newly opened issue is auto-labeled `triage`. The maintainer (or an agent acting on their behalf) reviews triage items and applies a routing decision in the same edit that removes `triage`.

By template:

- **`agent-task`**: verify the spec is complete (Context, Problem, Acceptance Criteria, Definition of Done, Files in scope, Files out of scope, Verification all filled; ACs are testable; file scope is concrete). Then either assign the issue to **Copilot**, add the **`openhands`** label, or leave unrouted for the maintainer. Remove `triage`.
- **`bug`**: try to reproduce. Add priority / area labels. If it's a clear, well-scoped fix, convert it into an `agent-task` (file a new issue with the spec; cross-link). Remove `triage`.
- **`enhancement`**: discuss and refine. If accepted, convert it into an `agent-task` with a full spec. If declined, close with a brief rationale. Remove `triage`.

Filter the inbox with: `is:issue is:open label:triage`.

## Agent task workflow

### Issues are the spec

An `agent-task` issue body **is** the specification. There is no separate spec document. The acting agent treats the issue body as ground truth and the issue comments as clarifications. PRs close issues with `Fixes #N`.

### Routing

- **Assign the issue to Copilot** → GitHub Copilot's coding agent picks it up and opens a draft PR.
- **Add the `openhands` label** (or comment `@openhands ...`) → OpenHands Cloud picks it up and opens a PR.
- **No agent label or assignment** → the maintainer handles it.

These three are mutually exclusive on any given issue. If you change your mind, remove the previous routing first.

### Plans live in the PR body

The acting agent's **first commit** edits the PR body to add a `## Plan` checklist — concrete, ordered steps small enough that each box is a meaningful commit. Subsequent commits tick boxes off. The PR template (`.github/pull_request_template.md`) reserves the section.

This keeps the spec (read-only context) separate from the plan (the agent's working artifact) and gives the reviewer a built-in progress bar.

### Acceptance Criteria vs Definition of Done

- **Acceptance Criteria** (per-issue): the testable conditions for *this specific* change. Authored by the spec author. The agent ticks them off as it satisfies them.
- **Definition of Done** (universal): the shippability gates every change must clear. Identical on every `agent-task` issue. Reproduced inline in the issue template so the agent reads it alongside the ACs.

Default Definition of Done:

- [ ] `go build ./...` passes locally
- [ ] `go test -race -count=1 ./...` passes locally
- [ ] `golangci-lint run` passes locally with zero issues
- [ ] Coverage gate passes locally: `go test -coverprofile=cover.out ./... && go run github.com/vladopajic/go-test-coverage/v2@v2.18.8 --config ./.testcoverage.yml` (CI enforces this — gate failure is a hard stop)
- [ ] PR title is a valid Conventional Commit, and is about *why* (not *what*)
- [ ] `## Why` in the PR body explains the motivation (it becomes the squash-merge commit body)
- [ ] Documentation updated if user-visible behavior or API changed
- [ ] AI-assistant attribution per AGENTS.md (no AI `Co-Authored-By:` trailers on commits *you* author; bot-authored Copilot/OpenHands PRs are exempt — bot as `Author` + human as `Co-authored-by:` is the agreed pattern)
- [ ] All Acceptance Criteria boxes ticked
- [ ] PR is **one concern**, reviewable in one sitting, independently revertable, with self-contained tests (LOC > ~200 is a warning sign — audit against these criteria and flag in the PR body with rationale if the overrun is justified)

Strike through (`~~- [ ] ...~~`) any DoD line that genuinely does not apply, with a one-line rationale. Removal of a line is an explicit choice that the reviewer can challenge.

### Verify before claiming done

Run the verification commands (from the issue's *Verification* section) and paste their relevant output into the PR description. "I think it works" is not enough; the reviewer should not have to re-derive that the change is correct.

### When an issue is too big for one PR

If an agent-task issue would exceed the ~200 LOC cap, **do not** push past it. Decompose it into **sub-issues** using GitHub's native sub-issues feature (GA April 2025): each child is a real hierarchical relation, GitHub renders a progress bar over them automatically, and the parent is implicitly a tracking root (no separate `tracking` label needed — the sub-issue relation itself signals "parent"). Where children have an order, express it with the native **`blocked by`** issue dependencies (GA August 2025) — triage queries the blocked-by edge and skips children whose blockers are still open.

This can happen at triage (spotted upfront, the canonical case) or mid-implementation (discovered while coding, the edge case). The exact procedure — including the GraphQL mutations needed since `gh issue` does not yet have CLI subcommands for sub-issues or dependencies — is in the [Decomposition](.agents/skills/triage.md#decomposition) section of the triage skill.

## Recipes

### Recipe: Adding a GitHub Action

1. Find the latest tagged release:
   ```sh
   gh release view --repo OWNER/REPO --json tagName -q .tagName
   ```
2. Resolve that tag to its commit SHA:
   ```sh
   gh api repos/OWNER/REPO/commits/<TAG> --jq .sha
   ```
3. Reference it in the workflow with the version as a trailing comment:
   ```yaml
   - uses: org/repo@<full-sha>  # vX.Y.Z
   ```
4. Dependabot will keep both the SHA and the comment current as new versions release.

## Anti-patterns

- **Bundling unrelated changes in one PR.** Open a separate issue and PR.
- **Skipping hooks** (`--no-verify`, `--no-gpg-sign`). Fix the underlying failure instead.
- **Floating tags on GitHub Actions** (`@v6`, `@main`). Always pin a SHA.
- **`Co-Authored-By:` trailers for AI assistants on commits *you* author.** Bot-authored PRs from Copilot / OpenHands are the exception — see *AI-assistant attribution* under PR conventions.
- **Speculative refactors during a fix.** A bug fix doesn't need surrounding cleanup; if cleanup is warranted, open a separate issue.
- **Editing files outside the issue's listed scope.** Comment on the issue and wait instead of silently expanding.
- **Inventing API or library facts.** If you cannot cite a source for a claim about how something works, do not include it.

## Per-agent notes

### Review coverage matrix

The shape of "who reviews whom" depending on the PR author, and what verdicts are available to the maintainer reviewer:

| Author     | Copilot auto-reviews? | Maintainer can `--approve` / `--request-changes`? |
|------------|-----------------------|---------------------------------------------------|
| Human      | yes                   | yes                                               |
| OpenHands  | yes                   | **no — GitHub blocks** (PR appears authored by the dispatching maintainer) |
| Copilot    | **no — self-review block** | yes                                           |

### Copilot

- Entry point: [`.github/copilot-instructions.md`](.github/copilot-instructions.md) (a thin pointer back to this file).
- Dev-environment bootstrap: [`.github/workflows/copilot-setup-steps.yml`](.github/workflows/copilot-setup-steps.yml) — installs Go and pre-downloads module deps in Copilot's ephemeral runner.
- **Copilot code-review** is enabled repo-wide (Settings → Code & automation → Code review). Copilot automatically posts a review when a PR is marked ready-for-review. Treat Copilot's review as an additional signal — a second pair of eyes from a different model layer — alongside the canonical human / Claude review that follows [`.agents/skills/review.md`](.agents/skills/review.md). The skill's review is the one that drives merge; Copilot's is input, not verdict.
- **Auto-review gap.** Copilot's auto-review does *not* fire on PRs authored by Copilot itself (Copilot cannot review its own PRs). On human-authored and OpenHands-authored PRs, Copilot auto-review provides a useful second signal (refs PR [#20](https://github.com/RemkoMolier/renovate-alpine-datasource/pull/20) where it caught three real issues). On Copilot-authored PRs, this signal is absent. **Decision: accept the gap** (option (c)). Copilot-authored PRs require explicit maintainer review with extra rigor — the [Five red-flag pass](.agents/skills/review.md#five-red-flag-pass-agent-prs-first) *and* the [Standard pass](.agents/skills/review.md#standard-pass-all-prs) without Copilot's second pair of eyes. Alternatives (dispatching OpenHands as cross-agent reviewer, third-party tools) were considered and declined as premature until Copilot-authored PR volume warrants them.
- **Conversation-resolution gate.** Copilot's auto-review threads must be resolved before merge (branch protection). When the reviewer-maintainer disagrees with a Copilot review finding, the workflow is: **reply with rationale, then resolve the thread**. Do not leave threads unresolved in the hope that Copilot will learn — it won't. Do not approve with unresolved review threads; the branch-protection gate will reject the merge. (See `.agents/skills/review.md` *Disputing Copilot auto-review threads* for the full procedure.)

### OpenHands

- Primary context: this file (`AGENTS.md`) — OpenHands reads it on every session.
- Session bootstrap: [`.openhands/setup.sh`](.openhands/setup.sh) — runs at the start of every session. The default OpenHands runtime image (python-nodejs) does not include Go, so the script installs the latest Go release from go.dev (only on first run; cached after that), then downloads Go module deps.
- Domain-specific knowledge that should only load when relevant lives under [`.agents/skills/`](.agents/skills/); load on demand.
- **Author identity.** The OpenHands GitHub App opens PRs **under the dispatching maintainer's identity** (the GitHub user who authorized the App and triggered the dispatch). GitHub therefore treats the PR's `author` field as the maintainer, even though the actual git commit `Author` on the branch is `openhands <openhands@all-hands.dev>`. This is different from Copilot, which opens PRs under its own bot identity (`copilot-swe-agent[bot]`).
- **Self-review block.** Because OpenHands PRs appear authored by the maintainer, GitHub blocks `Approve` and `Request Changes` reviews by that same maintainer — you cannot approve or block your own PR. Only `Comment` is available. The workaround is documented in [`.agents/skills/review.md`](.agents/skills/review.md): use `--comment` and label blocking findings as `Critical` / `Important` in the review body so the author (OpenHands) can act on them as if they were a Request-Changes verdict.
- **Branch protection consequence.** The `required_approving_review_count` branch-protection setting cannot be raised above 0 while OpenHands is an active routing option — a required review count of 1 would deadlock every OpenHands PR because the maintainer cannot satisfy it. This is accepted as a known limitation of the current OpenHands App architecture. If GitHub ever allows the App to open PRs under a separate bot identity, revisit.

## Skills

Procedural how-tos that are too long or too situational for this file live under `.agents/skills/<topic>.md`, written as plain agent-readable markdown. Current skills:

- [`idiomatic-go.md`](.agents/skills/idiomatic-go.md) — Go style rules and the most common pitfalls in this repo.
- [`triage.md`](.agents/skills/triage.md) — operational how-to for working through the `triage` inbox: spec verification, routing heuristics, and decomposition of oversized issues into sub-issues.
- [`tdd.md`](.agents/skills/tdd.md) — the Red → Green → Refactor loop, exception list, and how the (light) enforcement works.
- [`review.md`](.agents/skills/review.md) — reviewing a PR: the Five red-flag pass for agent-authored PRs, the Standard pass, comment vocabulary, and the exact `gh` invocations for a batched review.

Add a new skill when a procedure recurs across more than one issue or is too detailed to inline here. Keep each skill focused on one topic.
