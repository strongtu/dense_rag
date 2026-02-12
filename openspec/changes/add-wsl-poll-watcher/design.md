## Context

dense-rag runs in WSL2 and needs to monitor Windows-mounted directories (`/mnt/c/...`). The Linux kernel's inotify subsystem does not receive events through WSL2's 9P filesystem protocol, so fsnotify is ineffective for these paths. A polling fallback is required, scoped only to affected directories, while preserving the existing fsnotify watcher for native Linux paths.

### Stakeholders
- Users running dense-rag inside WSL2 who store documents on Windows drives

### Constraints
- Pure Go standard library only (no new third-party dependencies for the poll watcher)
- No Windows-side helper programs
- Must not disrupt existing behavior for non-WSL or native Linux directory monitoring

## Goals / Non-Goals

### Goals
- Support monitoring multiple directories simultaneously (mix of native Linux + Windows-mounted)
- Automatically select poll vs fsnotify per directory based on runtime environment detection
- Detect file add/modify/delete on `/mnt/` paths with acceptable latency (~10s)
- Backward-compatible config parsing (old `watch_dir: "..."` still works)

### Non-Goals
- Sub-second latency for polled directories (10s is acceptable for document workflows)
- Monitoring non-file changes (permissions, ownership, xattrs)
- Supporting file types beyond `.txt` / `.docx` (existing filter is sufficient)
- Windows-native watcher via named pipes or helper processes

## Decisions

### 1. Watcher interface extraction
- **Decision**: Extract a `DirWatcher` interface from the existing `Watcher` struct so both fsnotify-based and poll-based implementations satisfy the same contract.
- **Interface**: `Start(ctx context.Context) error` + `Stop()`
- **Rationale**: Keeps `main.go` agnostic to watcher implementation; each directory gets its own watcher instance.
- **Alternatives considered**: Single hybrid watcher that internally mixes inotify + polling — rejected because it increases coupling and makes testing harder.

### 2. Per-directory watcher assignment
- **Decision**: At startup, iterate `watch_dirs`. For each directory, if WSL is detected AND path starts with `/mnt/`, create a `PollWatcher`; otherwise create the existing fsnotify-based `NotifyWatcher`.
- **Rationale**: Even in WSL2, native ext4 paths (`~/Documents`) work fine with inotify. Only 9P-mounted paths need polling.
- **Alternatives considered**: Global polling when WSL detected — rejected because it unnecessarily degrades latency for native paths.

### 3. Polling strategy: full WalkDir per cycle, no directory ModTime shortcut
- **Decision**: Each poll cycle does a full `filepath.WalkDir` with suffix filter, comparing ModTime + Size against the previous snapshot.
- **Rationale**: Directory ModTime behavior on 9P/NTFS is unreliable (child changes may not propagate to parent directory timestamps). At 1000-2000 files and 10s intervals, full walk is cheap (~tens of ms).
- **Alternatives considered**: Check root directory ModTime first, skip walk if unchanged — rejected due to unreliable behavior on 9P.

### 4. Poll events go through Debouncer
- **Decision**: Poll-detected changes are submitted through the existing Debouncer before reaching the WorkerPool, same as fsnotify events.
- **Rationale**: Maintains architectural uniformity. If a file changes across two consecutive poll cycles (user still editing), the debouncer coalesces them.

### 5. Config backward compatibility via custom UnmarshalYAML
- **Decision**: Use a custom YAML unmarshaler that accepts both `watch_dir: "path"` (string) and `watch_dirs: ["path1", "path2"]` (array). If both are present, `watch_dirs` takes precedence.
- **Rationale**: Avoids breaking existing user configs. Migration is optional.

### 6. Unified reconciliation before watcher startup
- **Decision**: On startup, run `Reconcile()` across ALL configured directories (merged results), process the diff, then start individual watchers. PollWatcher initializes its snapshot from the same walk, preventing duplicate "added" events on first cycle.
- **Rationale**: Ensures consistent initial state regardless of watcher type.

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| 9P stat calls are slower than ext4 (~2-5x) | 10s interval keeps CPU impact low; suffix filter reduces stat count |
| ModTime precision loss through 9P | ModTime + Size dual check reduces false negatives; worst case is a missed update within same second and same size (extremely rare for documents) |
| Word temp files (`~$*.docx`) trigger false events | Add `~$` prefix exclusion to file filter |
| User configures overlapping directories | Validate at startup: reject if any dir is an ancestor of another |

## Open Questions
- None at this time; previous rounds of review have resolved all open items.
