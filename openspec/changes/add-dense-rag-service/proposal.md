# Change: Add Dense RAG Service

## Why
We need a local service that monitors file directories for changes in txt/docx files,
builds and maintains a vector index, and exposes an HTTP API for semantic search.
This enables fast, real-time retrieval-augmented generation over local documents.

## What Changes
- Add HTTP API module: query endpoint (`POST /query`) and health endpoint (`GET /health`)
- Add YAML-based configuration management (`~/.dense_rag/config.yaml`)
- Add recursive file watcher for txt/docx files with debounce and flow control
- Add data cleaning pipeline: docx-to-text extraction, chunking
- Add in-memory vector store with disk persistence and startup reconciliation
- Add embedding client compatible with OpenAI API (`/v1/embeddings`)

## Impact
- Affected specs: http-api, config-management, file-watcher, data-cleaning, vector-store, embedding-client
- Affected code: This is a greenfield project; all code is new
- External dependencies: fsnotify, gin/echo, yaml.v3, zakahan/docx2md, Go standard library
