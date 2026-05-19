# Reviewing a pull request

The procedure for reviewing a PR in this repository, whether the author is a human, Copilot, OpenHands, or the maintainer themselves. See [AGENTS.md](../../AGENTS.md) for the conventions a PR must conform to; this file is the operational how-to for the reviewer.

**Core principle: technical correctness over social comfort.** Do not approve to be polite; do not request changes to be thorough.

## Before you start

1. **Identify the author type** from the PR's author field:
   - A human (maintainer or contributor) → straight to the [Standard pass](#standard-pass).
   - `copilot-swe-agent[bot]` → Copilot coding agent → run the [Five red-flag pass](#five-red-flag-pass-agent-prs-first) first.
   - The OpenHands GitHub App → OpenHands Cloud → same red-flag pass first.
2. **Pull the branch and run the required gates locally** — reading the diff is not a substitute for seeing the code execute:
   ```sh
   gh pr checkout <n>
   make verify
   ```
3. **Build context.** Read the linked issue (`gh issue view <n>`), the PR body's `## Why` section, and the surrounding files the diff touches. *Understanding the change is the single biggest review challenge* (Bacchelli & Bird, ICSE 2013). Invest time here before commenting.

## Five red-flag pass (agent PRs first)

From GitHub's published guidance on reviewing agent-produced PRs. Each is a hard stop until addressed:

1. **CI Gaming.** Did any test get deleted, renamed, skipped (`t.Skip`), moved behind a build tag, or have its assertions weakened? Did `.golangci.yml`, `.testcoverage.yml` thresholds, or workflow `on:` triggers change? **A red `Coverage` check is a hard stop** — never approve a PR with a failing coverage gate; ask the author to add the missing tests (or, for exempt PR types, justify the exemption in `## Why`). Any CI-weakening blocks merge until justified and approved as a separate concern.
2. **Code reuse blindness.** `grep -r '<new helper name>' .` for every new helper, struct, or constant. Agents commonly reinvent existing utilities under slightly different names. If a duplicate exists, request consolidation.
3. **Hallucinated correctness.** For every external API call in the diff (stdlib or third-party), confirm the function exists and has the signature claimed. Agents fabricate plausible method names and parameter shapes. Require a test that *fails on the pre-change code and passes after* — and require the test output to be pasted into the PR body.
4. **Oversized / unscoped PR.** Apply the [splitting criteria from AGENTS.md](../../AGENTS.md#pr-conventions): one concern, reviewable in one sitting, independently revertable, self-contained tests. If any fail, **request a split rather than reviewing**.
5. **Untrusted input in workflows.** Does this PR cause text from PR titles, issue bodies, branch names, or fork-authored content to be interpolated into a prompt, an `eval`, or a shell command? Block until sanitised. Look specifically at additions in `.github/workflows/` and any new `gh api` invocations that pass user-controlled strings.

## Standard pass (all PRs)

Read every line. Cover these dimensions in order:

1. **Plan-vs-diff alignment.** The PR body's `## Plan` checklist should match the changes. Drift = scope creep; ask for either a plan update or a scope trim.
2. **Design.** Does this change fit how the rest of the code is shaped? Will it constrain future work?
3. **Functionality.** Does it do what the linked issue's Acceptance Criteria say? Trace at least one end-to-end path.
4. **TDD evidence.** The Coverage CI gate guarantees tests *exist*; the reviewer's job is to spot-check that the **test came first**. Skim the PR's commit list for a test-first commit on a behavior-changing PR. If absent, ask. If the PR is exempt (refactor / docs / build / test-only), expect the exception called out in the `## Why` section. See [tdd.md](./tdd.md).
5. **Tests substantively.** Do new tests actually exercise the new code paths, or just assert `err == nil` on happy paths? Are edge cases enumerated (zero, empty, nil, max, negative, concurrent)? Did anything get mocked that should be exercised for real?
6. **Go-specific checks** — see [Idiomatic Go](./idiomatic-go.md). At minimum:
   - Errors wrapped with `%w`; compared with `errors.Is` / `errors.As`, not `==`.
   - `context.Context` is the first arg of any I/O / blocking function and is actually propagated; no `context.TODO()` or `context.Background()` smuggled into request paths.
   - Every spawned goroutine has a documented exit (channel close, context done, finite work).
   - Public API has godoc on every exported identifier; doc starts with the identifier name.
7. **Naming & comments.** Names match scope. Comments explain *why*, not *what* — the code shows what.
8. **Definition of Done.** Every DoD checkbox in the linked issue is satisfied or struck through with rationale.
9. **Praise.** Leave at least one `praise:` comment per non-trivial PR. Reinforcing good patterns is documented review value (Google eng-practices; Bacchelli & Bird: knowledge transfer), not flattery.

## Comment vocabulary

Use [Conventional Comments](https://conventionalcomments.org) labels on every comment. Pair with `(blocking)` or `(non-blocking)` to make the verdict explicit:

- `praise:` — reinforce a good pattern. At least one per non-trivial PR.
- `issue (blocking):` — a defect or rule violation that must be addressed.
- `issue (non-blocking):` — a problem the author should know about, but not a merge blocker.
- `suggestion:` — a concrete alternative. Use ```suggestion blocks for changes ≤ a few lines so the author can commit your wording in one click.
- `question:` — clarification request. Must be answered (in PR body or via a code change — not just in the thread).
- `nitpick (non-blocking):` — taste-level. Always non-blocking.
- `todo:` — follow-up tracked as a new issue. File the issue and link it.

If you don't decorate a comment with `(blocking)`/`(non-blocking)`, the author will treat it as blocking.

## Output format

The top-level review body bundles all your findings into one notification. Use this template:

```markdown
### Strengths

- (at least one specific thing done well)

### Issues

#### Critical
- `path/to/file.go:42` — what's wrong / why it matters / how to fix.

#### Important
- ...

#### Minor
- ...

### Recommendations

- (optional broader feedback that didn't fit as a line comment)

### Assessment

Ready to merge? [Yes | No | With fixes]
```

Severity mapping:
- **Critical** — bugs, data loss, security regression, broken build, missing TDD evidence on a behavior change, any red flag from the Five red-flag pass. Blocks merge.
- **Important** — architecture concerns, missing edge-case tests, error-handling gaps, undocumented public API. Fix before merge.
- **Minor** — style, optimization, doc polish. Note, don't block.

## How to leave the review

`gh pr review` does not support inline line comments — use `gh api` to attach them as part of one batched review:

```sh
# Top-level body from a file (most common):
gh pr review <n> --request-changes --body-file review.md
# (Use --approve or --comment instead of --request-changes for the other verdicts.)

# Or, batched with inline line comments via the REST API:
gh api repos/$GH_OWNER/$GH_REPO/pulls/<n>/reviews -X POST \
  -f commit_id="$(git rev-parse HEAD)" \
  -f event=REQUEST_CHANGES \
  -f body="See line comments below." \
  -f 'comments[][path]=internal/foo.go' \
  -F 'comments[][line]=42' \
  -f 'comments[][side]=RIGHT' \
  -f 'comments[][body]=issue (blocking): nil-check missing.

```suggestion
if cfg == nil { return ErrNilConfig }
```'
```

**Always batch into one review** (`gh api .../reviews` or "Start a review" in the UI). Drip-feeding individual comments notifies the author N times for N findings and fragments the response.

Reply to an existing review thread:
```sh
gh api repos/$GH_OWNER/$GH_REPO/pulls/<n>/comments/<comment-id>/replies -X POST -f body='...'
```

## Verdicts

- **Approve** — "merge when CI is green." Use when the PR improves overall code health, even if not perfect; perfection is not the bar (Google eng-practices).
- **Comment** — feedback, no verdict. Use for partial reviews or when another reviewer is the decider.
- **Request changes** — "do not merge until resolved." Reserve for blocking issues. Never use Request Changes for nitpicks alone.

## Escalation

- A **Critical** finding on a security-sensitive path (auth, crypto, input parsing on untrusted data, secrets handling, `.github/workflows/`) → request a human maintainer review; do not approve as a single reviewer.
- The PR touches a path listed in the agent-task issue's *Files out of scope* → request a scope correction; do not approve.
- You and the author disagree → push back with technical reasoning, citing the specific rule from AGENTS.md or the skill being violated. Do not capitulate to keep the peace; do not stonewall to win.

## Forbidden

- "LGTM" without running the code locally.
- Approving when CI is red.
- Performative agreement ("Great PR!", "You're absolutely right!") with no specifics.
- Marking nitpicks as Critical (or Critical findings as nitpicks).
- Reviewing a PR over the size cap without first requesting a split.
- Drip-feeding single comments instead of batching one review.
- Approving an agent-produced PR without running the Five red-flag pass.
