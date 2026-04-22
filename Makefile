.PHONY: help test lint ci snapshot tools

help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "; printf "Usage: make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z_-]+:.*## / {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

test: ## Run unit tests
	go test ./...

lint: ## Run golangci-lint
	golangci-lint run --timeout 5m0s ./...

ci: lint test ## Run lint + test (used by CI)

snapshot: ## Build a local release snapshot via goreleaser
	goreleaser release --snapshot --clean --skip=publish

tools: ## Install goreleaser and golangci-lint
	go install github.com/goreleaser/goreleaser/v2@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
