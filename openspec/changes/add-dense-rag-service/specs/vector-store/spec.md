## ADDED Requirements

### Requirement: In-Memory Vector Storage
The system SHALL store all computed vectors in memory for fast retrieval.

The store SHALL maintain a mapping from file paths to their associated chunk vectors,
enabling efficient removal of all vectors belonging to a specific file.

#### Scenario: Add vectors for a file
- **WHEN** a file is processed and produces 10 chunk vectors
- **THEN** all 10 vectors are stored in memory with their associated file path and chunk text

#### Scenario: Remove vectors for a deleted file
- **WHEN** a file is deleted and had 10 chunk vectors in the store
- **THEN** all 10 vectors are removed from the store

### Requirement: Cosine Similarity Search
The system SHALL support top-k search using cosine similarity between a query
vector and all stored vectors.

Results SHALL be returned in descending order of similarity score.

#### Scenario: Top-k search
- **WHEN** a query vector is searched against a store containing 1000 vectors with topk=5
- **THEN** the 5 vectors with the highest cosine similarity are returned, ordered by descending score

### Requirement: Disk Persistence
The system SHALL periodically persist the in-memory vector store to disk using
Go's `encoding/gob` format, stored at `~/.dense_rag/store.gob`.

The persisted file SHALL include a schema version header for forward compatibility.

#### Scenario: Persistence on shutdown
- **WHEN** the service receives a shutdown signal (SIGINT/SIGTERM)
- **THEN** the current vector store state is saved to disk before the process exits

#### Scenario: Load from disk on startup
- **WHEN** the service starts and `~/.dense_rag/store.gob` exists
- **THEN** the vector store is loaded from disk into memory

### Requirement: Startup Reconciliation
The system SHALL reconcile the loaded vector store with the current state of the
watched directory on startup.

Files present in the store but missing on disk SHALL have their vectors removed.
Files present on disk but missing from the store SHALL be processed and indexed.
Files modified since last persistence (by mtime comparison) SHALL be re-processed.

#### Scenario: File deleted while service was stopped
- **WHEN** the service starts and a file tracked in the store no longer exists on disk
- **THEN** the vectors for that file are removed from the store

#### Scenario: New file added while service was stopped
- **WHEN** the service starts and a new `.txt` file exists in the watched directory but is not in the store
- **THEN** the file is processed, embedded, and its vectors are added to the store
