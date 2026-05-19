# renovate-alpine-datasource — local development targets.
#
# Tool versions below should be kept in sync with the SHA-pinned action
# versions in .github/workflows/ci.yml. When Dependabot bumps an action SHA
# there, the same PR should bump the matching version variable here.

GOLANGCI_LINT_VERSION    := v2.11.4
GO_TEST_COVERAGE_VERSION := v2.18.8

# Pinned `go run` invocations — no separate install step required;
# Go caches the binary after the first invocation.
GOLANGCI_LINT    := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
GO_TEST_COVERAGE := go run github.com/vladopajic/go-test-coverage/v2@$(GO_TEST_COVERAGE_VERSION)

.PHONY: help build vet test lint coverage fmt tidy verify

help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Compile all packages
	go build ./...

vet: ## Run go vet
	go vet ./...

test: ## Run tests with the race detector and no test result cache
	go test -race -count=1 ./...

lint: ## Run golangci-lint (pinned via $(GOLANGCI_LINT_VERSION))
	$(GOLANGCI_LINT) run

coverage: ## Run tests with coverage and enforce thresholds (.testcoverage.yml)
	go test -race -count=1 -coverprofile=cover.out ./...
	$(GO_TEST_COVERAGE) --config ./.testcoverage.yml

fmt: ## Format Go source
	go fmt ./...

tidy: ## Tidy go.mod / go.sum
	go mod tidy

verify: build vet test lint coverage ## Run every check that CI runs
	@echo "verify: all checks passed"
