## 1. Multi-directory config support
- [x] 1.1 Add `WatchDirs []string` field to `Config` struct in `internal/config/config.go`
- [x] 1.2 Implement custom YAML unmarshaling to support both `watch_dir` (string) and `watch_dirs` (array) formats, with `watch_dirs` taking precedence
- [x] 1.3 Update `DefaultConfig()` to use `WatchDirs: []string{"~/Documents"}`
- [x] 1.4 Apply `expandTilde` to all entries in `WatchDirs`; normalize with `filepath.Abs` + `filepath.Clean`
- [x] 1.5 Add directory overlap validation in `Validate()`: reject if any directory is an ancestor of another
- [x] 1.6 Update `configs/config.yaml` and `configs/config.example.yaml` to show `watch_dirs` array format
- [x] 1.7 Write unit tests for config parsing (old format, new format, both present, overlap detection)

## 2. WSL detection
- [x] 2.1 Create `internal/watcher/wsl.go` with `IsWSL() bool` function that reads `/proc/version` and checks for `microsoft`/`WSL` (case-insensitive)
- [x] 2.2 Create `NeedsPollWatcher(path string) bool` that returns true only when `IsWSL() && strings.HasPrefix(path, "/mnt/")`
- [x] 2.3 Write unit tests for WSL detection logic

## 3. Watcher interface extraction
- [x] 3.1 Define `DirWatcher` interface in `internal/watcher/watcher.go`: `Start(ctx context.Context) error` + `Stop()`
- [x] 3.2 Rename current `Watcher` struct to `NotifyWatcher`; ensure it satisfies `DirWatcher`
- [x] 3.3 Update `NewWatcher` → `NewNotifyWatcher` (or add a factory function); keep signature compatible
- [x] 3.4 Verify existing tests still pass after rename

## 4. Poll watcher implementation
- [x] 4.1 Create `internal/watcher/poll.go` with `PollWatcher` struct implementing `DirWatcher`
- [x] 4.2 Implement snapshot type: `map[string]fileState` where `fileState` holds `ModTime int64` and `Size int64`
- [x] 4.3 Implement `scan()` method: `filepath.WalkDir` with suffix filter + `~$` prefix exclusion, returns new snapshot
- [x] 4.4 Implement `diff()` method: compare current vs previous snapshot, produce `[]event{path, op}`
- [x] 4.5 Implement `Start(ctx)`: ticker loop (10s default), scan → diff → update snapshot → emit events through Debouncer → WorkerPool
- [x] 4.6 Implement `Stop()`: cancel context, release resources
- [x] 4.7 Support initializing PollWatcher with a pre-built snapshot (from reconciliation) to avoid first-cycle duplicate events
- [x] 4.8 Write unit tests for scan, diff, and lifecycle (use temp directories with known file states)

## 5. Temp file exclusion
- [x] 5.1 Update `cleaning.IsSupportedFile()` in `internal/cleaning/filter.go` to reject files with `~$` prefix (Word temp files)
- [x] 5.2 Write unit test for temp file exclusion

## 6. Reconciliation multi-directory support
- [x] 6.1 Update `Store.Reconcile()` in `internal/store/reconcile.go` to accept `[]string` (multiple directories) instead of single `string`
- [x] 6.2 Ensure reconciliation walk covers all directories and deduplicates results
- [x] 6.3 Update unit tests for multi-directory reconciliation

## 7. Main startup flow integration
- [x] 7.1 Update `main.go` to read `cfg.WatchDirs` and call `Reconcile` with all directories
- [x] 7.2 For each directory: call `NeedsPollWatcher(dir)` to decide watcher type
- [x] 7.3 Create `NotifyWatcher` or `PollWatcher` per directory; for PollWatcher pass initial snapshot from reconciliation scan
- [x] 7.4 Start all watchers in separate goroutines; wire shared `processFn` callback
- [x] 7.5 On shutdown: stop all watchers, save store
- [x] 7.6 Update log messages to show per-directory watcher type

## 8. Integration validation
- [ ] 8.1 Manual test: WSL2 environment with `/mnt/c/` directory — verify poll watcher detects add/modify/delete
- [ ] 8.2 Manual test: native Linux directory — verify fsnotify watcher still works
- [ ] 8.3 Manual test: mixed config with both native and `/mnt/` directories running simultaneously
- [ ] 8.4 Verify old `watch_dir: "~/Documents"` config format still works without changes
