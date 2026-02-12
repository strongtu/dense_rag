package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"dense-rag/internal/api"
	"dense-rag/internal/cleaning"
	"dense-rag/internal/config"
	"dense-rag/internal/embedding"
	"dense-rag/internal/store"
	"dense-rag/internal/watcher"
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

	// Ensure all watch directories exist.
	for _, dir := range cfg.WatchDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("create watch dir %s: %v", dir, err)
		}
	}

	// Startup reconciliation: scan all watch_dirs, diff with store, queue updates.
	added, removed, updated := st.Reconcile(cfg.WatchDirs, []string{".txt", ".docx"})
	processFile(st, embedClient, added, updated, removed)

	log.Printf("startup reconciliation complete: %d added, %d updated, %d removed",
		len(added), len(updated), len(removed))

	// processFn is shared by all watchers.
	processFn := func(path string, op watcher.EventOp) {
		switch op {
		case watcher.OpCreateModify:
			processFileEvent(st, embedClient, path)
		case watcher.OpDelete:
			st.Remove(path)
			log.Printf("removed: %s", path)
		}
	}

	// Create per-directory watchers.
	var watchers []watcher.DirWatcher
	for _, dir := range cfg.WatchDirs {
		if watcher.NeedsPollWatcher(dir) {
			// Build initial snapshot so the first poll cycle doesn't re-emit
			// files already processed during reconciliation.
			snap := watcher.BuildSnapshot(dir)
			pw := watcher.NewPollWatcher(dir, processFn, snap)
			watchers = append(watchers, pw)
			log.Printf("watch: %s (poll, 10s interval)", dir)
		} else {
			nw, err := watcher.NewNotifyWatcher(dir, processFn)
			if err != nil {
				log.Fatalf("create watcher for %s: %v", dir, err)
			}
			watchers = append(watchers, nw)
			log.Printf("watch: %s (fsnotify)", dir)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start all watchers.
	for _, w := range watchers {
		w := w
		go func() {
			if err := w.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("watcher error: %v", err)
			}
		}()
	}

	// Initialize and start HTTP API server.
	srv := api.NewServer(cfg, st, embedClient)
	go func() {
		if err := srv.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Fatalf("server error: %v", err)
		}
	}()

	stats := st.Stats()
	dirList := fmt.Sprintf("[%s]", strings.Join(cfg.WatchDirs, ", "))
	log.Printf("dense-rag started: listening on %s:%d, watching %s, model %s, %d indexed files",
		cfg.Host, cfg.Port, dirList, cfg.Model, stats.IndexedFiles)

	// Wait for interrupt signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("received %v, shutting down...", sig)

	// Stop all watchers (no new events).
	cancel()
	for _, w := range watchers {
		w.Stop()
	}

	// Save store to disk.
	if err := st.Save(storePath); err != nil {
		log.Printf("warning: failed to save store: %v", err)
	} else {
		log.Printf("store saved to %s", storePath)
	}

	// Gracefully shutdown HTTP server (5s timeout).
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("warning: server shutdown error: %v", err)
	}

	log.Println("dense-rag stopped")
}

// processFile handles startup reconciliation by processing added/updated files
// and removing deleted ones.
func processFile(st *store.Store, embedClient *embedding.Client, added, updated, removed []string) {
	// Process additions and updates.
	for _, path := range append(added, updated...) {
		processFileEvent(st, embedClient, path)
	}

	// Process removals.
	for _, path := range removed {
		st.Remove(path)
		log.Printf("reconcile removed: %s", path)
	}
}

// processFileEvent reads, cleans, chunks, embeds, and stores a single file.
func processFileEvent(st *store.Store, embedClient *embedding.Client, path string) {
	// Read file content.
	text, err := cleaning.ReadFile(path)
	if err != nil {
		log.Printf("read error for %s: %v", path, err)
		return
	}

	// Chunk the text.
	chunks := cleaning.ChunkText(text, path, cleaning.DefaultChunkSize, cleaning.DefaultOverlap)
	if len(chunks) == 0 {
		return
	}

	// Collect chunk texts for embedding.
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Text
	}

	// Embed all chunks.
	vectors, err := embedClient.Embed(context.Background(), texts)
	if err != nil {
		log.Printf("embedding error for %s: %v", path, err)
		return
	}

	if len(vectors) != len(chunks) {
		log.Printf("embedding count mismatch for %s: got %d, expected %d", path, len(vectors), len(chunks))
		return
	}

	// Build store entries.
	entries := make([]store.VectorEntry, len(chunks))
	for i, c := range chunks {
		entries[i] = store.VectorEntry{
			Vector:     vectors[i],
			Text:       c.Text,
			FilePath:   path,
			ChunkIndex: c.Index,
		}
	}

	// Get file mtime.
	info, err := os.Stat(path)
	if err != nil {
		log.Printf("stat error for %s: %v", path, err)
		return
	}

	st.Add(path, info.ModTime().Unix(), entries)
	log.Printf("indexed: %s (%d chunks)", path, len(chunks))
}
