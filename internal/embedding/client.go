package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

const (
	defaultBatchSize  = 32
	defaultTimeout    = 30 * time.Second
	maxRetries        = 3
	initialBackoff    = 1 * time.Second
)

// Client communicates with an OpenAI-compatible embeddings API.
type Client struct {
	endpoint   string
	model      string
	batchSize  int
	httpClient *http.Client
}

// EmbeddingRequest is the JSON body sent to the embeddings endpoint.
type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbeddingData holds a single embedding vector from the response.
type EmbeddingData struct {
	Embedding []float32 `json:"embedding"`
}

// EmbeddingResponse is the JSON body returned by the embeddings endpoint.
type EmbeddingResponse struct {
	Data []EmbeddingData `json:"data"`
}

// NewClient creates an embedding client pointing at the given endpoint.
// If batchSize <= 0 it defaults to 32.
func NewClient(endpoint, model string, batchSize int) *Client {
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	return &Client{
		endpoint:  endpoint,
		model:     model,
		batchSize: batchSize,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// Embed returns embedding vectors for the given texts. Texts are split into
// batches of batchSize and sent to the API. 5xx errors are retried with
// exponential backoff (up to 3 attempts).
func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	results := make([][]float32, 0, len(texts))

	for i := 0; i < len(texts); i += c.batchSize {
		end := i + c.batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		resp, err := c.embedBatch(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("embedding batch starting at index %d: %w", i, err)
		}

		for _, d := range resp.Data {
			results = append(results, d.Embedding)
		}
	}

	return results, nil
}

// EmbedSingle is a convenience wrapper that embeds a single text string.
func (c *Client) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
	vecs, err := c.Embed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("no embedding returned for input")
	}
	return vecs[0], nil
}

// Ping checks that the embedding endpoint is reachable.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return fmt.Errorf("creating ping request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ping request failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("ping returned status %d", resp.StatusCode)
	}
	return nil
}

// embedBatch sends a single batch to the API with retry logic for 5xx errors.
func (c *Client) embedBatch(ctx context.Context, batch []string) (*EmbeddingResponse, error) {
	reqBody := EmbeddingRequest{
		Model: c.model,
		Input: batch,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	url := c.endpoint + "/v1/embeddings"

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * initialBackoff
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			// Network errors are not necessarily retryable; but if context
			// is done, bail immediately.
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("reading response: %w", err)
			continue
		}

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("server error: status %d, body: %s", resp.StatusCode, string(respBody))
			continue
		}

		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("client error: status %d, body: %s", resp.StatusCode, string(respBody))
		}

		var embResp EmbeddingResponse
		if err := json.Unmarshal(respBody, &embResp); err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}

		return &embResp, nil
	}

	return nil, fmt.Errorf("all retries exhausted: %w", lastErr)
}
