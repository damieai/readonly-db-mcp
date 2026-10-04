package main

import (
	"github.com/your-org/readonly-db-mcp/internal/duckdbworker"
	"os"
)

func main() { os.Exit(duckdbworker.Run()) }
