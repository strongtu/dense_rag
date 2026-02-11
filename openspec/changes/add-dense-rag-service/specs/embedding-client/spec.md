## ADDED Requirements

### Requirement: OpenAI-Compatible Embedding Client
The system SHALL communicate with an external embedding service using the
OpenAI-compatible `/v1/embeddings` API protocol.

The client SHALL send HTTP POST requests with a JSON body containing `model`
and `input` fields, and parse the response to extract embedding vectors.

The default model SHALL be `text-embedding-bge-m3` with 1024-dimensional output.

#### Scenario: Single text embedding
- **WHEN** a single text string is sent for embedding
- **THEN** the client returns a 1024-dimensional float32 vector

#### Scenario: Embedding service error
- **WHEN** the embedding service returns an HTTP error (5xx)
- **THEN** the client retries with exponential backoff up to 3 times before returning an error

### Requirement: Batch Embedding
The system SHALL support sending multiple text inputs in a single embedding
request for improved throughput.

The batch size SHALL be configurable (default 32) and the client SHALL
automatically split large input sets into batches.

#### Scenario: Batch of 100 chunks
- **WHEN** 100 text chunks are submitted for embedding with batch_size=32
- **THEN** the client sends 4 requests (32+32+32+4) and returns 100 vectors

#### Scenario: Empty input
- **WHEN** an empty list of texts is submitted for embedding
- **THEN** the client returns an empty list of vectors without making any HTTP requests
