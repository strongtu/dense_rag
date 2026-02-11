## 1. Project Scaffolding
- [x] 1.1 Initialize Go module (`go mod init`), set up directory layout (`cmd/`, `internal/`, `configs/`)
- [x] 1.2 Add Makefile with `build`, `run`, `test`, `lint` targets
- [x] 1.3 Create default config file template at `configs/config.example.yaml`

## 2. Configuration Management (config-management)
- [x] 2.1 Define config struct with YAML tags (port, host, topk, watch_dir, model, model_endpoint)
- [x] 2.2 Implement config loader: read `~/.dense_rag/config.yaml`, merge defaults
- [x] 2.3 Add config validation (port range, directory exists, endpoint reachable)
- [x] 2.4 Write unit tests for config loading and defaults

## 3. Embedding Client (embedding-client)
- [x] 3.1 Implement OpenAI-compatible `/v1/embeddings` HTTP client
- [x] 3.2 Support batch embedding with configurable batch size
- [x] 3.3 Add retry with exponential backoff and timeout handling
- [x] 3.4 Write unit tests with mock HTTP server

## 4. Data Cleaning Pipeline (data-cleaning)
- [x] 4.1 Implement txt file reader (passthrough)
- [x] 4.2 Implement docx cleaner using `zakahan/docx2md` (strip images, tables, formulas, headers/footers)
- [x] 4.3 Implement text chunker (configurable chunk size and overlap)
- [x] 4.4 Add file size check (skip files >20MB)
- [x] 4.5 Write unit tests with sample txt and docx files

## 5. Vector Store (vector-store)
- [x] 5.1 Define vector store interface (Add, Remove, Search, Stats, Save, Load)
- [x] 5.2 Implement in-memory store with file→chunks mapping
- [x] 5.3 Implement brute-force cosine similarity search with top-k
- [x] 5.4 Implement gob-based disk persistence with schema version
- [x] 5.5 Implement startup reconciliation (load from disk → scan directory → diff → update)
- [x] 5.6 Write unit tests for CRUD, search accuracy, persistence round-trip

## 6. File Watcher (file-watcher)
- [x] 6.1 Implement recursive directory watcher using fsnotify
- [x] 6.2 Handle dynamic subdirectory creation/deletion (register/unregister watches)
- [x] 6.3 Filter events to txt/docx files only
- [x] 6.4 Implement event debounce (200ms window) to coalesce rapid changes
- [x] 6.5 Implement bounded worker pool for processing file events
- [x] 6.6 Wire watcher → cleaning → embedding → store pipeline
- [x] 6.7 Write integration tests with temp directory and file operations

## 7. HTTP API (http-api)
- [x] 7.1 Set up HTTP server with gin/echo
- [x] 7.2 Implement `POST /query` endpoint (accept text, return JSON with chunks, paths, scores)
- [x] 7.3 Implement `GET /health` endpoint (return store size, indexed file count, service status)
- [x] 7.4 Add request validation and error handling
- [x] 7.5 Write integration tests for both endpoints

## 8. Main Entrypoint & Integration
- [x] 8.1 Wire all modules in `cmd/dense-rag/main.go` (config → store → watcher → api)
- [x] 8.2 Implement graceful shutdown (stop watcher, flush store, close server)
- [x] 8.3 Add startup logging with config summary
- [x] 8.4 End-to-end manual test: start service, add/remove files, query API

## Dependencies
- Tasks 2 (config) is prerequisite for all other modules
- Task 3 (embedding) and 4 (cleaning) can run in parallel
- Task 5 (store) depends on 3 (embedding) for vector format
- Task 6 (watcher) depends on 4 (cleaning) and 5 (store)
- Task 7 (api) depends on 3 (embedding) and 5 (store)
- Task 8 (integration) depends on all previous tasks

## Parallelization
Team members can work concurrently on:
- **Member A**: Task 2 (config) → Task 6 (watcher)
- **Member B**: Task 3 (embedding) → Task 5 (store)
- **Member C**: Task 4 (cleaning) → Task 7 (api)
- **Member D**: Task 1 (scaffolding) → Task 8 (integration)
