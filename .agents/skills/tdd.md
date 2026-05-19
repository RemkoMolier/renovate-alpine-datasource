# Test-Driven Development

In this repository, **TDD is the norm for behavior-changing work**: write a failing test first, watch it fail (Red), write the minimum code to make it pass (Green), then improve the design with the safety net of a green test suite (Refactor).

## The loop

1. **Red.** Write the smallest test that captures the next slice of behavior you want. Run it. It must fail — for the right reason. If it passes immediately, the test wasn't testing what you thought.
2. **Green.** Write the *minimum* code that makes the test pass. Resist the urge to add anything not required by a test.
3. **Refactor.** With the test as a safety net, improve the design — rename, extract, simplify, remove duplication — keeping the test (and all others) green. Run the full suite (`go test -race -count=1 ./...`) after each refactor step.

Repeat. Each cycle should be small enough that the Red → Green transition is one commit and the Refactor is an optional follow-up commit, both small enough to review.

## What "behavior-changing" means

TDD applies when the PR adds, removes, or alters externally observable behavior. Concretely:

- A new feature, exported function, CLI flag, HTTP endpoint, or struct field.
- A bug fix: the reproducer test goes first (the bug becomes a failing test), then the fix turns it green. The test stays in the suite as a regression guard.

## Exceptions

These PRs do not require a new failing test (but must not weaken existing tests):

- **Refactor** — code restructured with no behavior change.
- **Docs** — README, godoc, AGENTS.md, skills, comments.
- **Build / CI / dependency bump** — workflow tweaks, `go.mod` updates, lint config changes.
- **Test-only** — adding tests for previously untested code (the test *is* the change).

When a PR falls into one of these, name the exception in the PR body's `## Why` section so the reviewer knows which gate applies.

## How this is enforced

Two layers — one mechanical, one cultural:

- **Mechanical: the Coverage CI gate.** The `Coverage` job runs `vladopajic/go-test-coverage` against the thresholds in [`.testcoverage.yml`](../../.testcoverage.yml) (currently `package: 80`). New packages without tests start at 0% coverage and fail the gate — so you cannot land new behavior without tests. Branch protection on `main` blocks merge while Coverage is red.
- **Cultural: the test-first ordering.** The coverage gate checks *that* tests exist, not *when* they were written. The TDD discipline — failing test in a commit *before* the implementing commit — is a cultural norm reviewers spot-check by scanning the PR's commits ([`review.md`](./review.md) Standard pass, item 4). Squash-merge erases the commit ordering on `main`, but the per-PR commit history is preserved on the PR forever; that's the audit trail.

If you find yourself wanting to skip the loop because "it's just a small change," that's usually exactly the case where TDD finds a corner you missed. Do the loop.

## Go-specific notes

- Tests are table-driven (`tests := []struct{ name string; in ...; want ... }{}` + `for _, tt := range tests { t.Run(tt.name, ...) }`). The failing-test commit typically adds a new table entry with the expected behavior; the implementing commit adds the production code that makes it pass.
- Use `t.Cleanup` for teardown — runs even on `t.FailNow`, clearer at the call site than `defer`.
- Always run with `-race -count=1` locally: `-race` catches concurrency bugs the type system can't, `-count=1` defeats Go's test result cache so you actually re-run the code.
- `t.Parallel()` is encouraged where safe; it also surfaces shared-state bugs faster.

## See also

- [Idiomatic Go](./idiomatic-go.md) — broader Go conventions, including testing.
- [Review](./review.md) — the reviewer's TDD-evidence check is documented under the Standard Pass.
