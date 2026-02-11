## 1. Scaffolding and helpers
- [x] 1.1 Create `test/integration_test.go` with package declaration, imports, and score threshold constants
- [x] 1.2 Implement `TestMain`: check embedding service reachability (GET model_endpoint/v1/models), build binary, defer cleanup
- [x] 1.3 Implement helper: `startService(t, watchDir) (baseURL, cleanup)` — generates temp config, starts binary subprocess, polls /health until ready, returns base URL and cleanup function
- [x] 1.4 Implement helper: `pollQuery(baseURL, text, matchFn, timeout) ([]ResultItem, error)` — polls POST /query every 500ms until matchFn returns true or timeout
- [x] 1.5 Implement helper: `pollHealth(baseURL, condFn, timeout) (HealthResponse, error)` — polls GET /health every 500ms until condFn returns true or timeout
- [x] 1.6 Implement helper: `queryOnce(baseURL, text) ([]ResultItem, error)` — single POST /query call
- [x] 1.7 Implement helper: `healthOnce(baseURL) (HealthResponse, error)` — single GET /health call

## 2. Core functionality tests (TC-01 ~ TC-04)
- [x] 2.1 Implement `TestCoreLifecycle` with shared service instance
- [x] 2.2 Subtest TC-01: create test_add.txt, poll query for semantic match, assert score > hit threshold
- [x] 2.3 Subtest TC-02: verify file_path is correct absolute path and file exists on disk
- [x] 2.4 Subtest TC-03: overwrite file with new content, poll for new content match, verify old content no longer matches
- [x] 2.5 Subtest TC-04: delete file, poll to verify content no longer retrievable

## 3. Health endpoint tests (TC-05 ~ TC-06)
- [x] 3.1 Implement `TestHealthEmpty` (TC-05): start with empty dir, verify status/vector_count/indexed_files fields, counts = 0
- [x] 3.2 Implement `TestHealthCountsChange` (TC-06): add files, poll health for count increases, delete file, poll for count decreases

## 4. Subdirectory tests (TC-07 ~ TC-08)
- [x] 4.1 Implement `TestSubdirectoryIndexing` (TC-07): create subdir + file, poll query, verify file_path includes subdir
- [x] 4.2 Implement `TestDynamicSubdirectory` (TC-08): create new subdir after service starts, add file, verify queryable

## 5. TopK test (TC-09)
- [x] 5.1 Implement `TestTopKLimit` (TC-09): create 6+ files with related content, query, assert result count <= 5

## 6. Boundary condition tests (TC-10 ~ TC-12)
- [x] 6.1 Implement `TestEmptyFile` (TC-10): create empty .txt, verify service stable, vector_count unchanged
- [x] 6.2 Implement `TestLargeFileIgnored` (TC-11): create >20MB file, verify not indexed
- [x] 6.3 Implement `TestUnsupportedFormats` (TC-12): create .pdf/.jpg/.csv, verify not indexed

## 7. Error input tests (TC-13 ~ TC-15)
- [x] 7.1 Implement `TestEmptyQueryText` (TC-13): POST with empty text, assert HTTP 400
- [x] 7.2 Implement `TestInvalidJSON` (TC-14): POST with invalid JSON body, assert HTTP 400
- [x] 7.3 Implement `TestQueryNoIndex` (TC-15): query with no indexed files, assert HTTP 200 with empty array

## 8. Data consistency tests (TC-16 ~ TC-17)
- [x] 8.1 Implement `TestRapidModification` (TC-16): rapid overwrites, verify only final content is queryable
- [x] 8.2 Implement `TestFileRename` (TC-17): create file, rename, verify new path in results, old path absent

## 9. Validation
- [x] 9.1 Run `go test ./test/... -v -count=1` with a live embedding service to verify all tests pass
- [x] 9.2 Verify test output format: PASS/FAIL per case, summary at end, nonzero exit on failure (standard Go test behavior)
