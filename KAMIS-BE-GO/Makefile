SERVICE ?= template

GOLANGCI_LINT_VERSION ?= v2.13.2
TOOLS_BIN := $(CURDIR)/bin/tools

.PHONY: tidy tidy-check run build test vet fmt fmt-check lint tools verify

tidy:
	go mod tidy

# tidy-check fails instead of rewriting, for CI. It catches a dependency added
# with `go get` and never tidied, which leaves a direct import sitting in the
# indirect block.
tidy-check:
	go mod tidy -diff

run:
	go run ./services/$(SERVICE)/cmd

build:
	CGO_ENABLED=0 go build -trimpath -o bin/$(SERVICE) ./services/$(SERVICE)/cmd

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

# fmt-check fails instead of rewriting, for CI.
fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "These files need gofmt:"; echo "$$unformatted"; exit 1; \
	fi

tools: $(TOOLS_BIN)/golangci-lint

$(TOOLS_BIN)/golangci-lint:
	GOBIN=$(TOOLS_BIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

lint: tools
	$(TOOLS_BIN)/golangci-lint run ./...

# verify is what CI runs, and what to run before pushing.
verify: fmt-check tidy-check vet lint test
