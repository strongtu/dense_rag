## Context
The dense-rag service is a Go application that watches a local directory for file changes, embeds text via an external embedding model, and serves semantic search over HTTP. Unit tests exist per package, but no integration tests validate the full pipeline.

The requirements document (`dense_rag_test_ai.txt`) specifies 17 test cases across 8 categories covering core CRUD, health endpoint, recursive subdirectory watching, topk limits, boundary conditions, error handling, and data consistency.

## Goals / Non-Goals
- **Goals:**
  - Implement all 17 test cases as Go integration tests runnable via `go test`
  - Tests are self-contained: create temp dirs, generate temp config, start/stop the service binary
  - Tests tolerate embedding latency via polling (not fixed sleeps)
  - Score thresholds are configurable constants at the top of the file
  - Final output: PASS/FAIL per test case, summary report, nonzero exit on failure
- **Non-Goals:**
  - Modifying any existing service source code
  - Testing .docx files (only .txt per requirements)
  - Performance/load testing
  - Testing WSL-specific polling watcher

## Decisions

### Decision 1: Black-box testing via compiled binary
The integration tests will build the dense-rag binary, then start it as a subprocess with a custom `--config` flag pointing to a temporary config file. This ensures the tests exercise the real startup path including config loading, store initialization, watcher setup, and HTTP server startup.

**Alternatives considered:**
- In-process integration (importing `main` internals): rejected because it wouldn't test the real binary startup and signal handling.
- Shell script tests (bash/curl): rejected because the requirements explicitly ask for Go automated test style, and Go tests provide better structure, parallelism control, and reporting.

### Decision 2: TestMain for shared setup/teardown
Use `TestMain` to:
1. Check embedding service availability (skip all tests if unreachable)
2. Build the binary once via `go build`
3. Clean up the binary on exit

Individual test functions manage their own temp directories and service instances to stay isolated.

### Decision 3: Shared service instance for sequential test groups
TC-01 through TC-04 operate on the same file and must run in order. These will be a single test function `TestCoreLifecycle` with subtests `t.Run("TC-01", ...)` through `t.Run("TC-04", ...)` sharing one service instance and watch directory.

All other test cases get their own service instance and temp directory for full isolation.

### Decision 4: Polling helper with configurable timeout
A `pollQuery` helper function will:
- Accept a query text, expected match condition, and timeout
- Send POST /query every 500ms
- Return the first response matching the condition, or error on timeout
- Default timeout: 10s (configurable per call)

Similarly, `pollHealth` will poll GET /health until a condition is met.

### Decision 5: Score thresholds
Define at file scope:
```go
const (
    ScoreHitThreshold  = 0.5  // score above this = match
    ScoreMissThreshold = 0.3  // score below this = no match
)
```
These can be tuned based on the bge-m3 model behavior.

## Risks / Trade-offs
- **Risk:** Embedding service may be slow or unavailable in CI → Mitigation: skip all tests with clear message if embedding is unreachable
- **Risk:** Score thresholds may need tuning per environment → Mitigation: constants at file top, easy to adjust
- **Risk:** Port conflicts if multiple tests run simultaneously → Mitigation: each service instance uses a unique random port
- **Trade-off:** Starting a full service process per test group is slower but more realistic than in-process testing

## Open Questions
- None at this time; all requirements are clearly specified in the test document.
