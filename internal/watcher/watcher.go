package watcher

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"dense-rag/internal/cleaning"
	"dense-rag/internal/config"

	"github.com/fsnotify/fsnotify"
)

// EventOp describes the type of file system event.
type EventOp int

const (
	OpCreateModify EventOp = iota
	OpDelete
)

const (
	DefaultDebounceWindow = 200 * time.Millisecond
	DefaultPoolSize       = 4
)

// Watcher monitors a directory tree for file changes and dispatches events
// through a debouncer and worker pool to a caller-provided processing function.
type Watcher struct {
	fsw       *fsnotify.Watcher
	cfg       *config.Config
	pool      *Pool
	debouncer *Debouncer
	processFn func(path string, op EventOp)
	cancel    context.CancelFunc
	done      chan struct{}
}

// NewWatcher creates a Watcher that watches cfg.WatchDir recursively.
// processFn is called (via the worker pool) for each debounced event.
func NewWatcher(cfg *config.Config, processFn func(path string, op EventOp)) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		fsw:       fsw,
		cfg:       cfg,
		pool:      NewPool(DefaultPoolSize),
		debouncer: NewDebouncer(DefaultDebounceWindow),
		processFn: processFn,
		done:      make(chan struct{}),
	}, nil
}

// Start begins watching the directory tree. It blocks until ctx is cancelled
// or an unrecoverable error occurs.
func (w *Watcher) Start(ctx context.Context) error {
	ctx, w.cancel = context.WithCancel(ctx)

	if err := w.addDirRecursive(w.cfg.WatchDir); err != nil {
		return err
	}

	defer close(w.done)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-w.fsw.Events:
			if !ok {
				return nil
			}
			w.handleEvent(event)
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return nil
			}
			log.Printf("watcher error: %v", err)
		}
	}
}

// Stop shuts down the watcher, debouncer, and worker pool.
func (w *Watcher) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	w.debouncer.Stop()
	w.fsw.Close()
	w.pool.Shutdown()
	// Wait for the event loop to finish.
	<-w.done
}

// addDirRecursive walks the directory tree and adds each directory to fsnotify.
func (w *Watcher) addDirRecursive(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		if info.IsDir() {
			if addErr := w.fsw.Add(path); addErr != nil {
				log.Printf("failed to watch %s: %v", path, addErr)
			}
		}
		return nil
	})
}

// handleEvent filters and dispatches a single fsnotify event.
func (w *Watcher) handleEvent(event fsnotify.Event) {
	path := event.Name

	// If a new directory is created, start watching it recursively.
	if event.Has(fsnotify.Create) {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			w.addDirRecursive(path)
			return
		}
	}

	// Only process supported file types.
	if !cleaning.IsSupportedFile(path) {
		return
	}

	// For delete/rename, debounce and dispatch as OpDelete.
	if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
		w.debouncer.Debounce(path, func() {
			w.pool.Submit(func() {
				w.processFn(path, OpDelete)
			})
		})
		return
	}

	// For create/write, check file size first.
	if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
		tooLarge, err := cleaning.IsFileTooLarge(path)
		if err != nil {
			return // file may have been removed between event and stat
		}
		if tooLarge {
			log.Printf("skipping %s: exceeds max file size", path)
			return
		}
		w.debouncer.Debounce(path, func() {
			op := OpCreateModify
			if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
				op = OpDelete
			}
			w.pool.Submit(func() {
				w.processFn(path, op)
			})
		})
	}
}
