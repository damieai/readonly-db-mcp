.PHONY: build test test-race fmt vet tidy

build:
	go build -trimpath -o bin/readonly-db-mcp ./cmd/readonly-db-mcp

test:
	go test ./...

test-race:
	go test -race ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

vet:
	go vet ./...

tidy:
	go mod tidy
