## What

<!-- Briefly describe the change. The diff shows what changed line-by-line; keep
     this to 1-3 lines that orient the reviewer. -->

## Why

<!-- The motivation. This is the most important section: explain why this change
     exists, what it enables or prevents, and what tradeoff you accepted. The
     squash-merge commit body inherits from here, so it is what future archaeologists
     will read when `git blame`-ing this code in 18 months.

     Link related issues with `Fixes #N` / `Refs #N`. -->

## Plan

<!--
For PRs implementing an agent task, draft the implementation plan as a checklist
here in your first commit, then tick items off as you go. Each box should be
small enough to be meaningful as a single commit.

Skip this section for trivial PRs (docs typo, one-line fix) by deleting it.

Example:
- [ ] Add APKINDEX parser skeleton in `internal/apk/index.go`
- [ ] Cover the parser with a table-driven test
- [ ] Wire parser into the HTTP handler
- [ ] Update README with the new endpoint
-->

## Verification

<!-- Commands you ran (and their relevant output) to demonstrate the change works.
     These must match the required gates from AGENTS.md exactly — do not abridge. -->

```sh
make verify
```

---

- [ ] PR title follows [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, `chore:`, ...)
- [ ] Tests added or updated
- [ ] Documentation updated if user-visible behavior or API changed
- [ ] PR is **one concern**, reviewable in one sitting, independently revertable, with self-contained tests (LOC > ~200 is a warning sign — audit and flag above with rationale if the overrun is justified)
