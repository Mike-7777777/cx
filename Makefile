.PHONY: build test lint vet clean coverage

BINARY = cx
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

build:
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o $(BINARY) .

test:
	go test ./... -count=1 -race

lint:
	golangci-lint run

vet:
	go vet ./...

coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

clean:
	rm -f $(BINARY) $(BINARY).exe coverage.out
