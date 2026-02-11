## ADDED Requirements

### Requirement: Recursive Directory Watching
The system SHALL monitor the configured `watch_dir` and all its subdirectories
recursively for file changes (create, modify, delete) to `.txt` and `.docx` files.

When a new subdirectory is created, the system SHALL automatically start monitoring it.
When a subdirectory is deleted, the system SHALL stop monitoring it and remove all
associated vectors from the store.

#### Scenario: New txt file created
- **WHEN** a new `.txt` file is created in the watched directory
- **THEN** the system reads the file, chunks it, computes embeddings, and adds vectors to the store

#### Scenario: Docx file modified
- **WHEN** an existing `.docx` file is modified in the watched directory
- **THEN** the system removes old vectors for the file, re-processes it, and adds updated vectors

#### Scenario: File deleted
- **WHEN** a monitored file is deleted
- **THEN** all vectors associated with that file are removed from the store

#### Scenario: New subdirectory created with files
- **WHEN** a new subdirectory containing `.txt` files is created under the watched directory
- **THEN** the system starts monitoring the subdirectory and indexes all qualifying files within it

### Requirement: Event Flow Control
The system SHALL implement flow control mechanisms to prevent excessive CPU and
memory usage during bulk file changes.

The system SHALL debounce file change events within a configurable time window
(default 200ms) to coalesce rapid successive changes to the same file.

The system SHALL use a bounded worker pool (default 4 workers) for processing
file change events.

#### Scenario: Rapid successive edits to the same file
- **WHEN** a file is saved 10 times within 200ms
- **THEN** the system processes the file only once after the debounce window expires

#### Scenario: Many files added simultaneously
- **WHEN** 500 files are copied into the watched directory at once
- **THEN** the system processes them through the worker pool without exceeding the configured concurrency limit

### Requirement: Large File Exclusion
The system SHALL ignore files larger than 20MB and not process or index them.

#### Scenario: File exceeds size limit
- **WHEN** a 25MB `.txt` file is created in the watched directory
- **THEN** the system logs a warning and does not process or index the file
