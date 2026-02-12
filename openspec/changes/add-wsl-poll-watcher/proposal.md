# Change: Add WSL2 poll-based file watcher for Windows mounted directories

## Why
dense-rag uses fsnotify (inotify) for file monitoring, which does not work on WSL2's 9P-mounted Windows directories (`/mnt/xxx`). Users running dense-rag in WSL2 who want to monitor Windows-side folders get no file change events. A polling-based watcher is needed as a fallback for these paths, alongside multi-directory configuration support.

## What Changes
- Config: `watch_dir` (string) becomes `watch_dirs` (string array), with backward compatibility for the old single-string format
- Config validation: detect and reject overlapping/nested directory configurations
- New `PollWatcher` in `internal/watcher/` that uses periodic `filepath.WalkDir` + ModTime/Size comparison to detect file changes on `/mnt/` paths
- WSL environment detection via `/proc/version`
- Startup flow: unified reconciliation across all directories, then each directory gets the appropriate watcher (fsnotify or poll) based on WSL detection + path prefix
- Both watcher types emit the same `EventOp` events into the shared Debouncer + WorkerPool pipeline

## Impact
- Affected specs: multi-dir-config (new), poll-watcher (new), wsl-detection (new)
- Affected code:
  - `internal/config/config.go` — struct change, parsing, validation
  - `configs/config.yaml`, `configs/config.example.yaml` — new format
  - `internal/watcher/` — new `poll.go`, watcher interface extraction
  - `cmd/dense-rag/main.go` — multi-directory reconcile + watcher startup loop
  - `internal/store/reconcile.go` — accept multiple directories
  - `internal/cleaning/filter.go` — add temp file exclusion (`~$` prefix)
