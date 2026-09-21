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

# SQL Server uses a separately packaged, self-contained ScriptDom process.
DOTNET ?= dotnet
SQLSERVER_RUNTIME ?= linux-x64

.PHONY: build-sqlserver-parser test-sqlserver-parser test-redis-local
build-sqlserver-parser:
	$(DOTNET) publish tools/sqlserver-parser -c Release -r $(SQLSERVER_RUNTIME) --self-contained true -p:RestoreLockedMode=true -o bin/sqlserver-parser

test-sqlserver-parser: build-sqlserver-parser
	READONLY_DB_MCP_SQLSERVER_PARSER="$(CURDIR)/bin/sqlserver-parser/readonly-sqlserver-parser" go test ./internal/dialects/sqlserver -run TestScriptDom -count=1

test-redis-local:
	test -n "$(READONLY_DB_MCP_REDIS_SERVER)"
	go test ./internal/dialects/redis -run TestLocalRedis -count=1 -v

.PHONY: test-redis-vectors
test-redis-vectors:
	test -n "$(READONLY_DB_MCP_REDIS_SERVER)"
	test -n "$(READONLY_DB_MCP_REDIS_SEARCH_MODULE)"
	test -n "$(READONLY_DB_MCP_REDIS_JSON_MODULE)"
	go test -race ./internal/mcpserver -run TestLocalRedisDenseVectorsThroughMCP -count=1 -v

.PHONY: build-pgvector-proof test-pgvector-proof
build-pgvector-proof:
	PG_CONFIG="$(PG_CONFIG)" sh tools/pgvector-proof/build.sh

test-pgvector-proof:
	test -n "$(READONLY_DB_MCP_PG_BIN)"
	test -n "$(READONLY_DB_MCP_PGVECTOR_PROOF)"
	go test -race ./internal/dialects/postgresql/vectorproof -count=1 -v
