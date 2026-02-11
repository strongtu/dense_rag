## ADDED Requirements

### Requirement: Integration Test Suite
The project SHALL provide an automated integration test suite in `test/integration_test.go` that validates the full dense-rag service pipeline (file watching, embedding, indexing, and HTTP query/health endpoints) against a live embedding service.

#### Scenario: Embedding service unavailable
- **WHEN** the embedding service is not reachable at test startup
- **THEN** all integration tests SHALL be skipped with a clear message indicating the embedding service is unavailable

#### Scenario: Test isolation
- **WHEN** integration tests are executed
- **THEN** each test group SHALL use a dedicated temporary directory as `watch_dir` and a unique port for the HTTP server
- **AND** all temporary files, directories, and service processes SHALL be cleaned up after tests complete (whether passing or failing)

### Requirement: Core CRUD Test Cases
The integration test suite SHALL include test cases TC-01 through TC-04 that validate the file lifecycle (create, read-back via query, modify, delete) and verify semantic search accuracy using score thresholds.

#### Scenario: New file is queryable (TC-01)
- **WHEN** a .txt file is created in the watch directory with semantically distinct content
- **THEN** a semantically related query SHALL return results with score above the hit threshold within the polling timeout

#### Scenario: Result contains correct file path (TC-02)
- **WHEN** a query returns results for an indexed file
- **THEN** the `file_path` field SHALL be the correct absolute path and the file SHALL exist on disk

#### Scenario: Modified file reflects new content (TC-03)
- **WHEN** a file's content is completely replaced
- **THEN** queries matching the new content SHALL return results with score above the hit threshold
- **AND** queries matching the old content SHALL NOT return results from that file above the hit threshold

#### Scenario: Deleted file is not queryable (TC-04)
- **WHEN** an indexed file is deleted from the watch directory
- **THEN** queries for that file's content SHALL NOT return results from that file path

### Requirement: Health Endpoint Test Cases
The integration test suite SHALL include test cases TC-05 and TC-06 that validate the `/health` endpoint reports accurate status, vector count, and indexed file count.

#### Scenario: Health check with empty directory (TC-05)
- **WHEN** the watch directory is empty
- **THEN** GET /health SHALL return JSON with `status`, `vector_count=0`, and `indexed_files=0`

#### Scenario: Health counts track file changes (TC-06)
- **WHEN** files are added to or removed from the watch directory
- **THEN** the `indexed_files` and `vector_count` values in /health SHALL increase or decrease accordingly

### Requirement: Subdirectory Watching Test Cases
The integration test suite SHALL include test cases TC-07 and TC-08 that validate recursive subdirectory monitoring.

#### Scenario: Existing subdirectory files are indexed (TC-07)
- **WHEN** a file is created in a subdirectory of the watch directory
- **THEN** the file SHALL be indexed and queryable, with `file_path` reflecting the subdirectory path

#### Scenario: Dynamically created subdirectory is watched (TC-08)
- **WHEN** a new subdirectory is created in the watch directory while the service is running, and a file is added to it
- **THEN** the file SHALL be indexed and queryable

### Requirement: TopK Limit Test Case
The integration test suite SHALL include test case TC-09 that validates query result count respects the configured topk limit.

#### Scenario: Results do not exceed topk (TC-09)
- **WHEN** more than topk files with semantically similar content are indexed
- **THEN** a query SHALL return at most topk results

### Requirement: Boundary Condition Test Cases
The integration test suite SHALL include test cases TC-10 through TC-12 that validate edge cases do not crash the service or pollute the index.

#### Scenario: Empty file does not crash service (TC-10)
- **WHEN** an empty .txt file (0 bytes) is created in the watch directory
- **THEN** the service SHALL remain healthy and the file SHALL NOT produce vector entries

#### Scenario: Large file is ignored (TC-11)
- **WHEN** a .txt file larger than 20MB is created in the watch directory
- **THEN** the file SHALL NOT be indexed

#### Scenario: Unsupported file formats are ignored (TC-12)
- **WHEN** files with unsupported extensions (.pdf, .jpg, .csv) are created in the watch directory
- **THEN** the files SHALL NOT be indexed

### Requirement: Error Input Test Cases
The integration test suite SHALL include test cases TC-13 through TC-15 that validate the /query endpoint rejects invalid input gracefully.

#### Scenario: Empty query text returns 400 (TC-13)
- **WHEN** POST /query is called with `{"text": ""}`
- **THEN** the response SHALL be HTTP 400

#### Scenario: Invalid JSON returns 400 (TC-14)
- **WHEN** POST /query is called with a non-JSON body
- **THEN** the response SHALL be HTTP 400

#### Scenario: Query with no index returns empty array (TC-15)
- **WHEN** POST /query is called with valid text but no files are indexed
- **THEN** the response SHALL be HTTP 200 with an empty JSON array

### Requirement: Data Consistency Test Cases
The integration test suite SHALL include test cases TC-16 and TC-17 that validate the service correctly handles rapid modifications and file renames.

#### Scenario: Rapid modifications converge to final state (TC-16)
- **WHEN** a file is overwritten multiple times in rapid succession (< 100ms intervals)
- **THEN** only the final content SHALL be queryable after the debounce window

#### Scenario: File rename updates index (TC-17)
- **WHEN** an indexed file is renamed (via `mv`)
- **THEN** query results SHALL reference the new file path
- **AND** query results SHALL NOT reference the old file path
