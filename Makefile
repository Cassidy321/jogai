.PHONY: dev test lint fmt build clean security

dev: ## Run in dev mode
	go run ./cmd/jogai

test: ## Run all tests with race detection and coverage
	go test ./... -race -cover

lint: ## Run linter
	golangci-lint run

fmt: ## Format code
	gofmt -w .
	goimports -w .

build: ## Build binary
	go build -o bin/jogai ./cmd/jogai

clean: ## Remove build artifacts
	rm -rf bin/ dist/

security: ## Run security checks
	govulncheck ./...

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
