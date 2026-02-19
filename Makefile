# GoPotency Makefile

.PHONY: test bench build clean lint help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

test: ## Run all tests
	go test -v -race ./...

bench: ## Run all benchmarks
	go test -v -bench=. -benchmem ./...

lint: ## Run golangci-lint (requires installation)
	golangci-lint run

build: ## Build examples
	go build -o bin/gin-basic ./examples/gin-basic/main.go
	# Add other examples here

clean: ## Remove binaries and test artifacts
	rm -rf bin/
	go clean -testcache
