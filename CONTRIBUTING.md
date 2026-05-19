# Contributing

Thanks for considering a contribution. This project is a [Renovate](https://docs.renovatebot.com/) datasource for Alpine Linux packages, intended for use via Renovate's [`customDatasources`](https://docs.renovatebot.com/modules/datasource/custom/) mechanism.

## Prerequisites

- Go 1.25 or later

We track the latest released Go version and the one before it (mirroring [Go's own support window](https://go.dev/doc/devel/release#policy)). When a new Go minor version is released, the floor moves up.

## Build and test

```sh
go build ./...
go test ./...
```

## Commit messages

We use [Conventional Commits](https://www.conventionalcommits.org/). The PR title becomes the squash-merge commit subject, so it must follow the format too.

Common prefixes:

- `feat:` — new functionality
- `fix:` — bug fix
- `docs:` — documentation only
- `chore:` — tooling, deps, repo housekeeping
- `refactor:` — internal change with no behaviour difference
- `test:` — test-only change

Add a scope when it helps: `feat(apk): support edge/testing repository`.

Breaking changes: append `!` to the type (`feat!: drop support for ...`) and explain the migration in the body.

## Pull requests

1. Fork and create a topic branch from `main`.
2. Make the change with tests.
3. Open a PR. CI must pass and the PR title must be a valid Conventional Commit.
4. Squash-merge is the default. Releases are cut from `main` via [GoReleaser](https://goreleaser.com/) once a tag is pushed.

## Filing issues

Use the issue forms — bug reports and feature requests have separate templates with the fields we need for triage. General Renovate questions are better asked in the [Renovate community](https://github.com/renovatebot/renovate/discussions).
