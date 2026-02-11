## ADDED Requirements

### Requirement: Query Endpoint
The system SHALL expose a `POST /query` HTTP endpoint that accepts a JSON body
containing a `text` field, converts it to a vector via the embedding service,
performs top-k similarity search, and returns matching results.

The response SHALL be JSON with the following structure per result:
- `text`: the matched text chunk
- `file_path`: absolute path of the source file on the local filesystem
- `score`: cosine similarity score (float, 0.0–1.0)

The number of results SHALL be controlled by the `topk` configuration parameter (default 5).

#### Scenario: Successful query
- **WHEN** a client sends `POST /query` with body `{"text": "search term"}`
- **THEN** the system returns HTTP 200 with a JSON array of up to `topk` results, each containing `text`, `file_path`, and `score`, ordered by descending `score`

#### Scenario: Empty query
- **WHEN** a client sends `POST /query` with an empty or missing `text` field
- **THEN** the system returns HTTP 400 with an error message

#### Scenario: No matching results
- **WHEN** a client sends a valid query but the vector store is empty
- **THEN** the system returns HTTP 200 with an empty JSON array

### Requirement: Health Endpoint
The system SHALL expose a `GET /health` endpoint that returns the current service
status in JSON format, including:
- `status`: "ok" or "degraded"
- `vector_count`: total number of vectors in the store
- `indexed_files`: number of files currently indexed
- `store_size_bytes`: approximate memory usage of the vector store

#### Scenario: Healthy service
- **WHEN** a client sends `GET /health` and the service is running normally
- **THEN** the system returns HTTP 200 with `status: "ok"` and current statistics

#### Scenario: Embedding service unreachable
- **WHEN** a client sends `GET /health` and the embedding service is not reachable
- **THEN** the system returns HTTP 200 with `status: "degraded"` and current statistics
