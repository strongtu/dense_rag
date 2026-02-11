## Context
Greenfield Go service for monitoring local directories and providing vector-based
semantic search over txt and docx files. Target scale: 1,000 files initially,
expandable to 10,000. Runs as a single-process local daemon.

## Goals / Non-Goals
- Goals:
  - Real-time file monitoring with automatic re-indexing
  - Sub-second vector search over local documents
  - Minimal configuration, sensible defaults
  - Persistent vector store surviving restarts
  - Clean separation of modules for independent development

- Non-Goals:
  - Distributed deployment or clustering
  - Support for file types beyond txt/docx
  - Built-in model serving (embedding model is external)
  - User authentication or multi-tenancy

## Architecture Overview

```
┌─────────────┐     ┌──────────────┐     ┌────────────────┐
│  HTTP API   │────▶│ Vector Store │◀────│ Embedding      │
│  (gin/echo) │     │ (in-memory)  │     │ Client         │
└─────────────┘     └──────┬───────┘     └───────▲────────┘
                           │                      │
                    ┌──────▼───────┐     ┌────────┴────────┐
                    │ Persistence  │     │ Data Cleaning   │
                    │ (gob/disk)   │     │ (chunk+extract) │
                    └──────────────┘     └───────▲─────────┘
                                                 │
                                         ┌───────┴─────────┐
                                         │ File Watcher    │
                                         │ (fsnotify)      │
                                         └─────────────────┘
```

### Module Boundaries
1. **config**: Load/validate YAML config, provide typed access
2. **watcher**: fsnotify-based recursive watcher, event dedup/debounce, worker pool
3. **cleaning**: docx extraction (via docx2md), text chunking
4. **embedding**: OpenAI-compatible HTTP client, batch support, retry
5. **store**: In-memory vector storage, cosine similarity search, gob persistence, file-to-chunk mapping
6. **api**: HTTP endpoints (query, health), JSON serialization

### Data Flow
1. Watcher detects file change → emits event
2. Event is debounced (200ms window) and dispatched to worker pool
3. Worker reads file → cleaning pipeline extracts text → chunker splits into chunks
4. Chunks sent to embedding client in batches
5. Vectors stored in memory store, old vectors for same file removed first
6. Store periodically persisted to disk
7. Query API receives text → embedding client → cosine search → return top-k results

## Decisions
- **HTTP framework**: gin — lightweight, well-documented, fast; alternatives: echo, stdlib
- **File watcher**: fsnotify — de facto standard in Go; handle new subdirectories manually
- **DOCX parsing**: zakahan/docx2md — recommended by requirements; fallback: pandoc CLI
- **Vector storage**: Custom in-memory with brute-force cosine similarity — sufficient for 1K-10K files; HNSW can be added later behind interface
- **Persistence**: encoding/gob — simple, fast for Go structs; versioned schema header
- **Embedding API**: Standard HTTP client with OpenAI-compatible protocol; use sashabaranov/go-openai or raw HTTP
- **Concurrency**: Bounded worker pool (default 4 workers) for file processing; rate limiter for embedding API calls

## Risks / Trade-offs
- **docx2md maturity**: Smaller library, may fail on complex docx → Mitigation: integration test suite, fallback to pandoc
- **Memory at 10K files**: ~2.5GB with 1024-dim float32 vectors → Mitigation: document limits; future: quantization or mmap
- **Brute-force search at scale**: Linear scan over 500K vectors → Mitigation: acceptable for 1K; add HNSW index interface for future
- **inotify limits on Linux**: Default 8192 watches → Mitigation: document sysctl tuning; monitor watch count
- **External model dependency**: Embedding service must be running → Mitigation: health check, retry with backoff, clear error messages

## Open Questions
- Should gse-based rerank be included in v1 or deferred to a follow-up proposal?
- Exact chunk size and overlap strategy (e.g., 512 tokens with 64 overlap)?
- Persistence sync strategy: periodic timer vs on-change with debounce?
