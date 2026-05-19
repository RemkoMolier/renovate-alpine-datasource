# Copilot instructions

Authoritative rules for any change in this repository live in [`AGENTS.md`](../AGENTS.md) at the repo root. **Read it first** — it covers the project overview, build/test/lint commands, coding conventions, PR conventions, don't-touch zones, and the agent task workflow.

Copilot-specific reminders that don't fit elsewhere:

- **Issues are the spec.** When assigned, follow the Acceptance Criteria and Definition of Done in the issue body exactly. Strike through any DoD line you cannot meet, with a one-line rationale in the PR description.
- **Draft your plan in the PR body** before you start coding. Use the `## Plan` section in the pull request template; tick items off via commits.
- **Stay within the listed files in scope.** If you find you need to touch something outside that list, post a comment on the issue explaining why and wait — do not silently expand the scope.
- **PRs are one concern, reviewable in one sitting, independently revertable, with self-contained tests.** PRs over ~200 LOC are a warning sign to audit against those criteria — split if any fail, or flag in the PR description with rationale if the overrun is justified (e.g. cohesive boilerplate).
- **Conventional Commits in the PR title** (the title becomes the squash-merge subject and is validated by CI). The title and the `## Why` body are about *why*, not *what* — the diff shows what.
- **TDD for behavior changes.** Write the failing test first; the implementation comes second. The coverage gate (see CI) enforces that new code is tested. Refactor / docs / build / dep-bump / test-only PRs are exempt — call out the exception in the `## Why` section.
- **Commit attribution.** It's fine that your commits land with `Author: Copilot` and the assigning human appended as `Co-authored-by:` — that's the agreed pattern in AGENTS.md (PR conventions → *AI-assistant attribution*). Do **not** additionally add `Co-Authored-By: copilot-swe-agent` trailers — they're redundant when you're already the `Author` and bloat the squash-merge commit body.
