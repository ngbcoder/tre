BINARY := tre
PKG    := github.com/ngbcoder/tre
DIST   := dist

.PHONY: all build test race bench cover lint fmt vet tidy clean install run help

all: fmt vet test build ## Format, vet, test and build

build: ## Build the binary for the current platform
	go build -o $(BINARY) .

install: ## Install into $GOPATH/bin
	go install .

test: ## Run the test suite (git conformance tests included)
	go test ./...

test-short: ## Run tests without the git conformance suite
	go test -short ./...

race: ## Run the test suite under the race detector
	go test -race ./...

bench: ## Run benchmarks
	go test -bench=. -benchmem -run='^$$' ./...

cover: ## Report test coverage
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

fmt: ## Format all sources
	gofmt -w .

vet: ## Run go vet
	go vet ./...

tidy: ## Tidy go.mod / go.sum
	go mod tidy

dist: ## Cross-compile release binaries into ./dist
	@mkdir -p $(DIST)
	@for target in windows/amd64 darwin/amd64 darwin/arm64 linux/amd64 linux/arm64; do \
		os=$${target%/*}; arch=$${target#*/}; \
		ext=""; if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		echo "building $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags="-s -w" \
			-o $(DIST)/$(BINARY)_$${os}_$${arch}$$ext . || exit 1; \
	done

clean: ## Remove build artifacts
	rm -rf $(DIST) $(BINARY) $(BINARY).exe coverage.out

run: build ## Build then run against the current directory
	./$(BINARY)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
