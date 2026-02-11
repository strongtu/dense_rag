# Change: Add end-to-end integration test suite for dense-rag service

## Why
The dense-rag service currently only has unit tests per package. There is no automated end-to-end validation that the full lifecycle works correctly — file watcher picks up changes, embedding is computed, and `/query` + `/health` endpoints return accurate results. An integration test suite is needed to verify all 17 test cases specified in the requirements document (`dense_rag_test_ai.txt`).

## What Changes
- Add a single Go integration test file `test/integration_test.go` implementing TC-01 through TC-17
- The test suite manages its own dense-rag process lifecycle (start/stop) using a temporary config and temporary watch directory
- Embedding service availability is checked before any tests run; tests are skipped if unavailable
- Score-based pass/fail thresholds are used to account for vector search imprecision
- Polling-based waits replace fixed sleeps for robustness

## Impact
- Affected specs: none (no behavior change to the service itself)
- Affected code: new file `test/integration_test.go`, possibly a small test helper in `test/helpers_test.go`
- No changes to existing source code; tests exercise the binary as a black-box
