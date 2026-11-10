BINARY := serverku
PKG := ./cmd/serverku

.DEFAULT_GOAL := help

.PHONY: help build test test-race cover fmt vet lint tidy clean check brand-check brand-build brand-verify brand-rebuild brand-goldens

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

brand-check: ## Check brand tokens, roles, integrity and palette mutations (stdlib only)
	python3 scripts/check_brand.py
	python3 scripts/test_brand.py

brand-build: ## Rebuild the kit (install pinned Python/npm dependencies first)
	python3 brand/kit/source/build_kit.py

brand-verify: brand-check ## Verify artwork, font licences, icons and PDF (Python dependencies)
	python3 brand/kit/source/verify_kit.py

brand-rebuild: ## Check two isolated rebuilds against the committed kit
	python3 scripts/check_brand_rebuild.py

brand-goldens: ## Render guide samples with Poppler for manual visual review
	python3 scripts/render_brand_goldens.py

check: brand-check fmt vet test ## Check brand, format, vet, and test (run before pushing)

clean: ## Remove build artifacts
	rm -f $(BINARY) coverage.out
