package watcher

import (
	"context"
	"io/fs"
	"log"
	"path/filepath"
	"strings"
	"time"

	"dense-rag/internal/cleaning"
)

const DefaultPollInterval = 5 * time.Minute

// fileState holds the metadata used to detect file changes between poll cycles.
type fileState struct {
	ModTime int64
	Size    int64
}

// PollWatcher monitors a directory tree by periodic scanning and comparing
// file metadata against a cached snapshot. It emits events through a shared
// Debouncer and WorkerPool, compatible with NotifyWatcher's pipeline.
type PollWatcher struct {
	dir       string
	interval  time.Duration
	snapshot  map[string]fileState
	pool      *Pool
	debouncer *Debouncer
	processFn func(path string, op EventOp)
	cancel    context.CancelFunc
	done      chan struct{}
}

// NewPollWatcher creates a PollWatcher for the given directory.
// initialSnapshot can be provided (e.g. from reconciliation) to avoid
// duplicate events on the first poll cycle. Pass nil to start with an empty snapshot.
func NewPollWatcher(dir string, processFn func(path string, op EventOp), initialSnapshot map[string]fileState) *PollWatcher {
	snap := initialSnapshot
	if snap == nil {
		snap = make(map[string]fileState)
	}
	return &PollWatcher{
		dir:       dir,
		interval:  DefaultPollInterval,
		snapshot:  snap,
		pool:      NewPool(DefaultPoolSize),
		debouncer: NewDebouncer(DefaultDebounceWindow),
		processFn: processFn,
		done:      make(chan struct{}),
	}
}

// Start begins the polling loop. It blocks until ctx is cancelled.
func (pw *PollWatcher) Start(ctx context.Context) error {
	ctx, pw.cancel = context.WithCancel(ctx)
	defer close(pw.done)

	ticker := time.NewTicker(pw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			pw.pollOnce()
		}
	}
}

// Stop cancels the polling loop and waits for it to finish.
func (pw *PollWatcher) Stop() {
	if pw.cancel != nil {
		pw.cancel()
	}
	pw.debouncer.Stop()
	pw.pool.Shutdown()
	<-pw.done
}

// pollOnce performs a single scan-diff-emit cycle.
func (pw *PollWatcher) pollOnce() {
	current := pw.scan()
	events := pw.diff(current)
	// Update snapshot immediately, before event processing completes.
	pw.snapshot = current

	for _, ev := range events {
		path := ev.path
		op := ev.op
		pw.debouncer.Debounce(path, func() {
			pw.pool.Submit(func() {
				pw.processFn(path, op)
			})
		})
	}
}

type pollEvent struct {
	path string
	op   EventOp
}

// scan walks the directory tree and returns a snapshot of all supported files.
func (pw *PollWatcher) scan() map[string]fileState {
	result := make(map[string]fileState)

	filepath.WalkDir(pw.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("poll scan error: %s: %v", path, err)
			return nil // skip inaccessible paths
		}
		if d.IsDir() {
			return nil
		}
		if !cleaning.IsSupportedFile(path) {
			return nil
		}
		// Skip Word temp files (~$*.docx).
		base := filepath.Base(path)
		if strings.HasPrefix(base, "~$") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			log.Printf("poll stat error: %s: %v", path, err)
			return nil
		}

		result[path] = fileState{
			ModTime: info.ModTime().Unix(),
			Size:    info.Size(),
		}
		return nil
	})

	return result
}

// diff compares current scan results against the previous snapshot and returns
// a list of change events.
func (pw *PollWatcher) diff(current map[string]fileState) []pollEvent {
	var events []pollEvent

	// Detect additions and modifications.
	for path, cur := range current {
		prev, exists := pw.snapshot[path]
		if !exists {
			events = append(events, pollEvent{path: path, op: OpCreateModify})
		} else if cur.ModTime != prev.ModTime || cur.Size != prev.Size {
			events = append(events, pollEvent{path: path, op: OpCreateModify})
		}
	}

	// Detect deletions.
	for path := range pw.snapshot {
		if _, exists := current[path]; !exists {
			events = append(events, pollEvent{path: path, op: OpDelete})
		}
	}

	return events
}

// BuildSnapshot walks the given directory and returns a snapshot suitable for
// initializing a PollWatcher. This is typically called after reconciliation
// to avoid duplicate events on the first poll cycle.
func BuildSnapshot(dir string) map[string]fileState {
	result := make(map[string]fileState)

	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !cleaning.IsSupportedFile(path) {
			return nil
		}
		base := filepath.Base(path)
		if strings.HasPrefix(base, "~$") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		result[path] = fileState{
			ModTime: info.ModTime().Unix(),
			Size:    info.Size(),
		}
		return nil
	})

	return result
}
