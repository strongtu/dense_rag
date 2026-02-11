package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	embeddingpkg "dense-rag/internal/embedding"

	"dense-rag/internal/config"
	"dense-rag/internal/store"
)

// newTestSetup creates a Server backed by a fake embedding endpoint.
func newTestSetup(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()

	// Fake embedding server that returns a fixed vector.
	embedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/embeddings" {
			var req embeddingpkg.EmbeddingRequest
			json.NewDecoder(r.Body).Decode(&req)

			data := make([]embeddingpkg.EmbeddingData, len(req.Input))
			for i := range req.Input {
				data[i] = embeddingpkg.EmbeddingData{
					Embedding: []float32{0.1, 0.2, 0.3},
				}
			}
			resp := embeddingpkg.EmbeddingResponse{Data: data}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		// Ping endpoint
		w.WriteHeader(http.StatusOK)
	}))

	embedClient := embeddingpkg.NewClient(embedSrv.URL, "test-model", 0)

	st := store.NewStore()
	st.Add("test.txt", 1000, []store.VectorEntry{
		{Vector: []float32{0.1, 0.2, 0.3}, Text: "hello world", ChunkIndex: 0},
		{Vector: []float32{0.9, 0.8, 0.7}, Text: "goodbye world", ChunkIndex: 1},
	})

	cfg := &config.Config{
		Host: "127.0.0.1",
		Port: 0,
		TopK: 5,
	}

	srv := NewServer(cfg, st, embedClient)
	return srv, embedSrv
}

func TestQuerySuccess(t *testing.T) {
	srv, embedSrv := newTestSetup(t)
	defer embedSrv.Close()

	body := `{"text": "hello"}`
	req := httptest.NewRequest(http.MethodPost, "/query", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp QueryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp) == 0 {
		t.Fatal("expected at least one result")
	}

	// The first result should be "hello world" since its vector [0.1,0.2,0.3]
	// has perfect cosine similarity with the query vector.
	if resp[0].Text != "hello world" {
		t.Errorf("expected first result 'hello world', got %q", resp[0].Text)
	}
}

func TestQueryEmptyText(t *testing.T) {
	srv, embedSrv := newTestSetup(t)
	defer embedSrv.Close()

	body := `{"text": ""}`
	req := httptest.NewRequest(http.MethodPost, "/query", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestQueryInvalidJSON(t *testing.T) {
	srv, embedSrv := newTestSetup(t)
	defer embedSrv.Close()

	req := httptest.NewRequest(http.MethodPost, "/query", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHealthOK(t *testing.T) {
	srv, embedSrv := newTestSetup(t)
	defer embedSrv.Close()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}
	if resp.VectorCount != 2 {
		t.Errorf("expected 2 vectors, got %d", resp.VectorCount)
	}
	if resp.IndexedFiles != 1 {
		t.Errorf("expected 1 indexed file, got %d", resp.IndexedFiles)
	}
}

func TestHealthDegraded(t *testing.T) {
	// Use a server that is already closed to simulate unreachable embedding service.
	embedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	embedSrv.Close() // close immediately

	embedClient := embeddingpkg.NewClient(embedSrv.URL, "test-model", 0)
	st := store.NewStore()
	cfg := &config.Config{
		Host: "127.0.0.1",
		Port: 0,
		TopK: 5,
	}
	srv := NewServer(cfg, st, embedClient)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Status != "degraded" {
		t.Errorf("expected status 'degraded', got %q", resp.Status)
	}
}

func TestQueryEmptyStore(t *testing.T) {
	embedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := embeddingpkg.EmbeddingResponse{
			Data: []embeddingpkg.EmbeddingData{
				{Embedding: []float32{0.1, 0.2, 0.3}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer embedSrv.Close()

	embedClient := embeddingpkg.NewClient(embedSrv.URL, "test-model", 0)
	st := store.NewStore()
	cfg := &config.Config{Host: "127.0.0.1", Port: 0, TopK: 5}
	srv := NewServer(cfg, st, embedClient)

	body := `{"text": "hello"}`
	req := httptest.NewRequest(http.MethodPost, "/query", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp QueryResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp) != 0 {
		t.Errorf("expected empty results, got %d", len(resp))
	}
}
