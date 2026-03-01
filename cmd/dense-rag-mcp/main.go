package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"dense-rag/internal/config"
	"dense-rag/internal/embedding"
	"dense-rag/internal/mcp"
	"dense-rag/internal/store"
)

func main() {
	configPath := flag.String("config", "", "path to config file (default ~/.dense_rag/config.yaml)")
	flag.Parse()

	// Load configuration.
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	// Ensure data directory exists.
	dataDir := filepath.Join(os.Getenv("HOME"), ".dense_rag")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}
	storePath := filepath.Join(dataDir, "store.gob")

	// Initialize store and try loading persisted data.
	st := store.NewStore()
	if err := st.Load(storePath); err != nil {
		if !errors.Is(err, os.ErrNotExist) && !os.IsNotExist(err) {
			log.Printf("warning: could not load store from disk: %v", err)
		}
	}

	// Initialize embedding client.
	embedClient := embedding.NewClient(cfg.ModelEndpoint, cfg.Model, 0)

	// Create and start MCP server.
	mcpServer := mcp.NewMCPServer(st, embedClient, cfg.TopK)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("dense-rag MCP server started: model %s, %d indexed files", 
		cfg.Model, st.Stats().IndexedFiles)

	// Start MCP server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := mcpServer.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			errCh <- err
		}
	}()

	// Wait for interrupt signal or error
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("received %v, shutting down...", sig)
	case err := <-errCh:
		log.Printf("MCP server error: %v", err)
	}

	// Graceful shutdown
	cancel()
	log.Println("dense-rag MCP server stopped")
}