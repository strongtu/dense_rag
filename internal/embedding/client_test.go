package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestEmbedSingle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		var req EmbeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decoding request: %v", err)
		}
		if req.Model != "test-model" {
			t.Errorf("unexpected model: %s", req.Model)
		}
		if len(req.Input) != 1 || req.Input[0] != "hello world" {
			t.Errorf("unexpected input: %v", req.Input)
		}

		resp := EmbeddingResponse{
			Data: []EmbeddingData{
				{Embedding: []float32{0.1, 0.2, 0.3}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-model", 0)
	vec, err := client.EmbedSingle(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("EmbedSingle error: %v", err)
	}

	if len(vec) != 3 {
		t.Fatalf("expected 3 dimensions, got %d", len(vec))
	}
	if vec[0] != 0.1 || vec[1] != 0.2 || vec[2] != 0.3 {
		t.Errorf("unexpected vector: %v", vec)
	}
}

func TestEmbedBatch(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		var req EmbeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decoding request: %v", err)
		}

		data := make([]EmbeddingData, len(req.Input))
		for i := range req.Input {
			data[i] = EmbeddingData{
				Embedding: []float32{float32(i), float32(i + 1)},
			}
		}

		resp := EmbeddingResponse{Data: data}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// batchSize=2, 5 texts => 3 requests (2+2+1)
	client := NewClient(server.URL, "test-model", 2)

	texts := []string{"a", "b", "c", "d", "e"}
	vecs, err := client.Embed(context.Background(), texts)
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}

	if len(vecs) != 5 {
		t.Fatalf("expected 5 vectors, got %d", len(vecs))
	}
	if int(requestCount.Load()) != 3 {
		t.Errorf("expected 3 requests, got %d", requestCount.Load())
	}
}

func TestEmbedEmpty(t *testing.T) {
	client := NewClient("http://unused", "test-model", 0)
	vecs, err := client.Embed(context.Background(), []string{})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	if len(vecs) != 0 {
		t.Errorf("expected empty slice, got %d vectors", len(vecs))
	}
}

func TestEmbedRetryOn5xx(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
			return
		}

		var req EmbeddingRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := EmbeddingResponse{
			Data: []EmbeddingData{
				{Embedding: []float32{1.0}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-model", 0)
	// Override the backoff by using a small initial time - we test the logic
	// not the exact timing, so we accept the default backoffs here.
	// For fast tests we rely on the server responding quickly.

	vecs, err := client.Embed(context.Background(), []string{"retry me"})
	if err != nil {
		t.Fatalf("Embed error after retries: %v", err)
	}
	if len(vecs) != 1 {
		t.Fatalf("expected 1 vector, got %d", len(vecs))
	}
	if int(attempts.Load()) != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestEmbedRetriesExhausted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("always failing"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-model", 0)

	_, err := client.Embed(context.Background(), []string{"fail"})
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
}

func TestEmbedTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow server
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-model", 0)
	client.httpClient.Timeout = 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := client.Embed(ctx, []string{"timeout"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestPing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-model", 0)
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping error: %v", err)
	}
}

func TestPingFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-model", 0)
	if err := client.Ping(context.Background()); err == nil {
		t.Fatal("expected ping error for 503")
	}
}
