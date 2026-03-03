package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dense-rag/internal/config"
	embeddingpkg "dense-rag/internal/embedding"
	"dense-rag/internal/mcp"
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

// ---------------------------------------------------------------------------
// MCP (POST /mcp) tests — same Server, same test harness as /query and /health.
// ---------------------------------------------------------------------------

func TestMCPInitialize(t *testing.T) {
	srv, embedSrv := newTestSetup(t)
	defer embedSrv.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp mcp.MCPResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode MCP response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("MCP error: %d %s", resp.Error.Code, resp.Error.Message)
	}
	result, _ := resp.Result.(map[string]interface{})
	if result == nil {
		t.Fatal("expected result object")
	}
	if serverInfo, ok := result["serverInfo"].(map[string]interface{}); !ok || serverInfo["name"] != "dense-rag" {
		t.Errorf("expected serverInfo.name dense-rag, got %v", result["serverInfo"])
	}
}

func TestMCPToolsList(t *testing.T) {
	srv, embedSrv := newTestSetup(t)
	defer embedSrv.Close()

	body := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp mcp.MCPResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode MCP response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("MCP error: %d %s", resp.Error.Code, resp.Error.Message)
	}
	result, _ := resp.Result.(map[string]interface{})
	tools, _ := result["tools"].([]interface{})
	if len(tools) < 2 {
		t.Fatalf("expected at least 2 tools, got %d", len(tools))
	}
}

func TestMCPGetStats(t *testing.T) {
	srv, embedSrv := newTestSetup(t)
	defer embedSrv.Close()

	body := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_stats","arguments":{}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp mcp.MCPResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode MCP response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("MCP error: %d %s", resp.Error.Code, resp.Error.Message)
	}
	result, _ := resp.Result.(map[string]interface{})
	content, _ := result["content"].([]interface{})
	if len(content) == 0 {
		t.Fatal("expected content in get_stats result")
	}
	first, _ := content[0].(map[string]interface{})
	if text, ok := first["text"].(string); !ok || text == "" {
		t.Errorf("expected content[0].text string, got %v", first["text"])
	}
}

func TestMCPToolsCallSemanticSearch(t *testing.T) {
	srv, embedSrv := newTestSetup(t)
	defer embedSrv.Close()

	body := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"semantic_search","arguments":{"query":"hello","top_k":5}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp mcp.MCPResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode MCP response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("MCP error: %d %s", resp.Error.Code, resp.Error.Message)
	}
	result, _ := resp.Result.(map[string]interface{})
	content, _ := result["content"].([]interface{})
	if len(content) == 0 {
		t.Fatal("expected content in semantic_search result")
	}
	first, _ := content[0].(map[string]interface{})
	text, _ := first["text"].(string)
	if text == "" || !strings.Contains(text, "hello world") {
		t.Errorf("expected result text to mention 'hello world', got %q", text)
	}
}

func TestMCPInvalidJSON(t *testing.T) {
	srv, embedSrv := newTestSetup(t)
	defer embedSrv.Close()

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
