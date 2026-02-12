## ADDED Requirements

### Requirement: Poll-based file watcher
The system SHALL provide a poll-based file watcher that detects file additions, modifications, and deletions by periodically scanning the watched directory tree.

#### Scenario: File added
- **WHEN** a new supported file appears in the watched directory between two poll cycles
- **THEN** the system SHALL emit an OpCreateModify event for that file

#### Scenario: File modified
- **WHEN** a supported file's ModTime or Size changes between two poll cycles
- **THEN** the system SHALL emit an OpCreateModify event for that file

#### Scenario: File deleted
- **WHEN** a supported file present in the previous snapshot is no longer on disk
- **THEN** the system SHALL emit an OpDelete event for that file

### Requirement: Poll interval
The poll watcher SHALL scan the directory tree at a configurable interval, defaulting to 10 seconds (10000ms).

#### Scenario: Default interval
- **WHEN** the poll watcher starts with default configuration
- **THEN** it SHALL perform a full directory scan every 10 seconds

### Requirement: Suffix-based file filtering
The poll watcher SHALL only track files matching the supported file extensions (`.txt`, `.docx`) and SHALL skip all other files and directories during traversal.

#### Scenario: Unsupported file ignored
- **WHEN** a `.pdf` file is added to the watched directory
- **THEN** the poll watcher SHALL NOT emit any event for that file

#### Scenario: Temporary file excluded
- **WHEN** a file named `~$document.docx` exists in the watched directory
- **THEN** the poll watcher SHALL NOT track or emit events for that file

### Requirement: Snapshot-based change detection
The poll watcher SHALL maintain an in-memory snapshot of the previous scan cycle (file path to ModTime + Size mapping) and detect changes by comparing the current scan against this snapshot.

#### Scenario: Snapshot update after scan
- **WHEN** a poll cycle completes
- **THEN** the snapshot SHALL be replaced with the current scan results immediately, before event processing completes

### Requirement: Unified event pipeline
The poll watcher SHALL emit events using the same `EventOp` type (`OpCreateModify`, `OpDelete`) as the fsnotify-based watcher and route them through the shared Debouncer and WorkerPool.

#### Scenario: Event pipeline consistency
- **WHEN** the poll watcher detects a file change
- **THEN** the event SHALL pass through the Debouncer and WorkerPool before reaching the processing function, identical to fsnotify events

### Requirement: Graceful shutdown
The poll watcher SHALL stop its polling loop when the context is cancelled and release all resources.

#### Scenario: Context cancellation
- **WHEN** the parent context is cancelled
- **THEN** the poll watcher SHALL exit its scan loop within one poll interval and return

### Requirement: Error resilience
The poll watcher SHALL continue operating if individual file or directory stat calls fail during a scan cycle.

#### Scenario: Inaccessible file during scan
- **WHEN** a file cannot be stat'd during a poll cycle (e.g., permission denied, file locked)
- **THEN** the poll watcher SHALL skip that file, log a warning, and continue scanning remaining files

### Requirement: Unified startup reconciliation
The system SHALL perform reconciliation across ALL configured watch directories before starting any watcher. The poll watcher SHALL initialize its snapshot from the reconciliation scan results to avoid emitting duplicate events on its first cycle.

#### Scenario: Startup with poll watcher directory
- **WHEN** the system starts with a `/mnt/c/docs` directory assigned to the poll watcher
- **THEN** reconciliation SHALL scan `/mnt/c/docs`, process diffs against the store, and the poll watcher SHALL begin with a pre-populated snapshot matching the current disk state

### Requirement: No new third-party dependencies
The poll watcher SHALL be implemented using only the Go standard library, with no additional external dependencies.

#### Scenario: Dependencies unchanged
- **WHEN** the poll watcher is added to the codebase
- **THEN** `go.mod` SHALL NOT contain any new direct dependencies related to the poll watcher
