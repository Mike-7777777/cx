.PHONY: build test lint vet clean coverage check ci fmt tidy install-hooks

BINARY = cx
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# ── Build ───────────────────────────────────────────────────────────
build:
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o $(BINARY) .

# ── Quality checks (individual) ────────────────────────────────────
fmt:
	@UNFORMATTED=$$(gofmt -l .); \
	if [ -n "$$UNFORMATTED" ]; then \
		echo "gofmt: not formatted:"; echo "$$UNFORMATTED"; exit 1; \
	fi

tidy:
	@go mod tidy && \
	if ! git diff --quiet go.mod go.sum 2>/dev/null; then \
		git checkout go.mod go.sum 2>/dev/null; \
		echo "go.mod/go.sum not tidy — run 'go mod tidy'"; exit 1; \
	fi

vet:
	go vet ./...

test:
	go test ./... -count=1 -race

lint:
	golangci-lint run

# ── Aggregate targets ──────────────────────────────────────────────
# check: fast local gate (~15s, no external tools needed)
check: fmt tidy vet test build
	@echo "All checks passed."

# ci: full CI parity (requires golangci-lint)
ci: fmt tidy vet lint test build
	@echo "Full CI passed."

# ── Coverage ────────────────────────────────────────────────────────
coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

# ── Git hooks ───────────────────────────────────────────────────────
install-hooks:
	@git config core.hooksPath scripts/hooks
	@echo "Git hooks activated (scripts/hooks/)"

# ── Cleanup ─────────────────────────────────────────────────────────
clean:
	rm -f $(BINARY) $(BINARY).exe coverage.out
