GO_FILES := $(shell find . -name '*.go' -not -name '*_test.go')
PACKAGES  := $(shell go list ./...)

.PHONY: all build test lint vet fmt tidy docs clean help

default: build
	@echo "Build succeeded. Run tests via 'make test'. Show help via 'make help'"

help: ## Show this help
	@grep -Eh '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | sort | awk -F ':.*## ' '{printf "  %-20s %s\n", $$1, $$2}'

all: build test ## Build and run tests

build: $(GO_FILES) ## Build all packages
	go build ./...

test: ## Run unit tests
	go test ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format source files
	gofmt -l -w $(GO_FILES)

lint: vet ## Run static checks (currently just go vet)

tidy: ## Tidy go.mod/go.sum
	go mod tidy

docs: ## Regenerate package documentation in docs/
	go run github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest --output 'docs/{{.Dir}}.md' ./...

clean: ## Remove build artifacts
	go clean ./...
