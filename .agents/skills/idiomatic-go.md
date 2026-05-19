# Idiomatic Go

Authoritative external references — read these for the full picture:

- [Effective Go](https://go.dev/doc/effective_go)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- [Google Go Style Guide](https://google.github.io/styleguide/go/)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)

The rest of this file is **project-specific deltas and recurring pitfalls** in this repository. When in doubt, defer to the references above.

## Types

- Use `any`, never `interface{}` (Go 1.18+).
- Don't simulate inheritance via struct embedding. Compose with fields unless you genuinely want method promotion.
- Don't define interfaces preemptively. **Define them at the consumer**, with the smallest method set the consumer needs.

## Errors

- Wrap with `%w`: `fmt.Errorf("parsing apkindex: %w", err)`. Do not include the word "error" in the wrapping message — `%w` already adds context.
- Compare with `errors.Is` / `errors.As`. Never `err == someErr`.
- Sentinel errors are exported as `ErrFoo`; structured errors are `*FooError`.
- Do not `panic` outside `main` / `init`. Return an error.
- Errors are returned as the **last** value. Never via an out-parameter.

## Tests

- Table-driven, with `t.Run(name, func(t *testing.T) { ... })` for subtest reporting and `-run` filtering.
- Use `t.Cleanup` over `defer` for test teardown — runs even on `t.FailNow`, clearer at the call site.
- Test files are siblings of the code they cover (`foo.go` + `foo_test.go`), same package. Internal helpers live in `export_test.go`.
- Standard library `testing` is sufficient. Don't add `testify` (or any test dep) without an issue justifying it.

## Concurrency

- `context.Context` is the **first** argument of any function that does I/O, network, blocking sleeps, or anything that should be cancellable.
- Don't store `context.Context` in a struct field. Pass it through call arguments.
- Every goroutine you start must have a defined termination: closed channel, context done, finite work. No fire-and-forget.
- For concurrent fan-out, prefer `golang.org/x/sync/errgroup` over hand-rolled `sync.WaitGroup` + error channels.

## Naming

- Package names: lowercase, single word, no underscores, no `camelCase`. Match the directory's final segment.
- Exported names: `MixedCaps`. Acronyms stay all-caps (`HTTPHandler`, not `HttpHandler`).
- Don't stutter: `apk.Package`, not `apk.APKPackage`.
- Variable names get longer with scope: `i` in a tight loop, `idx` in a function, `packageIndex` at file scope.

## Imports

- Three groups separated by blank lines, in this order:
  1. Standard library
  2. Third-party
  3. Internal (this module)
- `goimports` is enabled in `.golangci.yml` and enforces the grouping. Run `golangci-lint run` before committing.

## What the linter already enforces

`.golangci.yml` already catches these — don't waste effort double-checking by hand:

- Unused variables, parameters, writes (`unused`, `unparam`).
- Vet analyzers: shadowing, printf format mismatches, lost cancel, atomic misuse, etc. (`govet` with `enable-all`).
- `gofmt -s` simplifications and `goimports` ordering.
- Go 1.22+ modernizations: `for i := range n` over `for i := 0; i < n; i++` (`intrange`), and the loop-var capture fix (`copyloopvar`).
- `stdlib` consts where applicable (`usestdlibvars`).

If `golangci-lint run` is green, the style basics are covered. Spend review attention on logic, error paths, and test coverage instead.
