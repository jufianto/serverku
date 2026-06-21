BINARY := serverku
PKG := ./cmd/serverku

.DEFAULT_GOAL := help

.PHONY: help build test test-race cover fmt vet lint tidy clean check

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		sort | awk 'BEGIN {FS = ":.*?## "} {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build: ## Build the CLI binary
	go build -o $(BINARY) $(PKG)

test: ## Run all tests
	go test ./...

test-race: ## Run all tests with the race detector
	go test -race ./...

cover: ## Run tests and print a coverage summary
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -n 1

fmt: ## Format all Go code
	gofmt -w .

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint (install: https://golangci-lint.run/usage/install)
	golangci-lint run

tidy: ## Tidy go.mod / go.sum
	go mod tidy

check: fmt vet test ## Format, vet, and test (run before pushing)

clean: ## Remove build artifacts
	rm -f $(BINARY) coverage.out
