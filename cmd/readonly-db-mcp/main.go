package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/your-org/readonly-db-mcp/internal/audit"
	"github.com/your-org/readonly-db-mcp/internal/config"
	"github.com/your-org/readonly-db-mcp/internal/mcpserver"
	"github.com/your-org/readonly-db-mcp/internal/registry"
)

var version = "dev"

func main() {
	configPath := flag.String("config", os.Getenv("READONLY_DB_MCP_CONFIG"), "path to YAML configuration")
	check := flag.Bool("check", false, "validate configuration and database permissions, then exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if *configPath == "" {
		logger.Error("configuration is required", "hint", "pass -config or set READONLY_DB_MCP_CONFIG")
		os.Exit(2)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("configuration rejected", "error", err)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	targets, err := registry.Open(ctx, cfg, audit.New(logger))
	if err != nil {
		logger.Error("startup safety verification failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := targets.Close(); err != nil {
			logger.Error("target shutdown failed", "error", err)
		}
	}()

	if *check {
		logger.Info("configuration and target permissions verified", "target_count", len(targets.List()))
		return
	}
	logger.Info("readonly database MCP started", "version", version, "target_count", len(targets.List()), "transport", "stdio")
	if err := mcpserver.New(targets, logger, version).Run(ctx); err != nil && ctx.Err() == nil {
		logger.Error("MCP server stopped", "error", err)
		os.Exit(1)
	}
}
