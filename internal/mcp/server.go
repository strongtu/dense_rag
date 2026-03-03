package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"dense-rag/internal/cleaning"
	"dense-rag/internal/embedding"
	"dense-rag/internal/store"
)

const maxDocumentSize = 5 << 20 // 5MB for get_document

// MCPServer implements the Model Context Protocol server for dense_rag
type MCPServer struct {
	store       *store.Store
	embedClient *embedding.Client
	topK        int
	input       io.Reader
	output      io.Writer
}

// NewMCPServer creates a new MCP server instance
func NewMCPServer(st *store.Store, embedClient *embedding.Client, topK int) *MCPServer {
	return &MCPServer{
		store:       st,
		embedClient: embedClient,
		topK:        topK,
		input:       os.Stdin,
		output:      os.Stdout,
	}
}

// MCPRequest represents a generic MCP request
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse represents a generic MCP response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// InitializeParams represents the initialize request parameters
type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
}

// ClientInfo represents client information
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult represents the initialize response
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
}

// Capabilities represents server capabilities
type Capabilities struct {
	Tools map[string]interface{} `json:"tools"`
}

// ServerInfo represents server information
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolsListResult represents the tools/list response
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// Tool represents a tool definition
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema represents the input schema for a tool
type InputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required"`
}

// ToolsCallParams represents the tools/call request parameters
type ToolsCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolsCallResult represents the tools/call response
type ToolsCallResult struct {
	Content []Content `json:"content"`
}

// Content represents content in a tool response
type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Start starts the MCP server and handles requests
func (s *MCPServer) Start(ctx context.Context) error {
	scanner := bufio.NewScanner(s.input)
	
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		line := scanner.Text()
		if line == "" {
			continue
		}
		
		var req MCPRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			log.Printf("MCP: failed to parse request: %v", err)
			continue
		}
		
		resp := s.handleRequest(ctx, &req)
		
		respJSON, err := json.Marshal(resp)
		if err != nil {
			log.Printf("MCP: failed to marshal response: %v", err)
			continue
		}
		
		fmt.Fprintln(s.output, string(respJSON))
	}
	
	return scanner.Err()
}

// HandleRequest processes an MCP JSON-RPC request and returns a response.
// It is used by both stdio and HTTP transports.
func (s *MCPServer) HandleRequest(ctx context.Context, req *MCPRequest) *MCPResponse {
	return s.handleRequest(ctx, req)
}

// handleRequest processes an MCP request and returns a response
func (s *MCPServer) handleRequest(ctx context.Context, req *MCPRequest) *MCPResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	default:
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}
}

// handleInitialize handles the initialize request
func (s *MCPServer) handleInitialize(req *MCPRequest) *MCPResponse {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Tools: map[string]interface{}{},
		},
		ServerInfo: ServerInfo{
			Name:    "dense-rag",
			Version: "1.0.0",
		},
	}
	
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// handleToolsList handles the tools/list request
func (s *MCPServer) handleToolsList(req *MCPRequest) *MCPResponse {
	tools := []Tool{
		{
			Name: "semantic_search",
			Description: "Search for semantically similar text chunks in the indexed documents. Returns a JSON array in result.content[0].text; each item has: text (matched snippet), file_path (absolute path on server, use with get_document to fetch full file), score (cosine similarity 0~1). Example: [{\"text\":\"...\",\"file_path\":\"/path/to/file.txt\",\"score\":0.92}]",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The search query text",
					},
					"top_k": map[string]interface{}{
						"type":        "integer",
						"description": "Number of top results to return (optional, default from config)",
						"minimum":     1,
						"maximum":     100,
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "get_stats",
			Description: "Get statistics about the indexed documents and vectors",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]interface{}{},
				Required:   []string{},
			},
		},
		{
			Name:        "get_document",
			Description: "Get the full text content of an indexed file. Use the file_path returned by semantic_search so the server (which has the files) can return the content for the agent to analyze.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "Exact file path as returned by semantic_search (path on the server machine)",
					},
				},
				Required: []string{"file_path"},
			},
		},
	}
	
	result := ToolsListResult{Tools: tools}
	
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// handleToolsCall handles the tools/call request
func (s *MCPServer) handleToolsCall(ctx context.Context, req *MCPRequest) *MCPResponse {
	var params ToolsCallParams
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}
	}
	
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}
	}
	
	switch params.Name {
	case "semantic_search":
		return s.handleSemanticSearch(ctx, req.ID, params.Arguments)
	case "get_stats":
		return s.handleGetStats(req.ID)
	case "get_document":
		return s.handleGetDocument(req.ID, params.Arguments)
	default:
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: "Tool not found",
			},
		}
	}
}

// handleSemanticSearch handles the semantic_search tool call
func (s *MCPServer) handleSemanticSearch(ctx context.Context, id interface{}, args map[string]interface{}) *MCPResponse {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error: &MCPError{
				Code:    -32602,
				Message: "Missing or invalid 'query' parameter",
			},
		}
	}
	
	topK := s.topK
	if topKArg, ok := args["top_k"].(float64); ok {
		topK = int(topKArg)
		if topK < 1 || topK > 100 {
			topK = s.topK
		}
	}
	
	// Embed the query
	vec, err := s.embedClient.EmbedSingle(ctx, query)
	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error: &MCPError{
				Code:    -32603,
				Message: "Embedding failed: " + err.Error(),
			},
		}
	}
	
	// Search for similar vectors
	results := s.store.Search(vec, topK)

	// Return structured JSON (same shape as HTTP POST /query) so agent can parse: [{ "text", "file_path", "score" }, ...]
	type resultItem struct {
		Text     string  `json:"text"`
		FilePath string  `json:"file_path"`
		Score    float32 `json:"score"`
	}
	items := make([]resultItem, len(results))
	for i, r := range results {
		items[i] = resultItem{Text: r.Text, FilePath: r.FilePath, Score: r.Score}
	}
	jsonBytes, _ := json.Marshal(items)
	content := []Content{{Type: "text", Text: string(jsonBytes)}}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  &ToolsCallResult{Content: content},
	}
}

// handleGetDocument returns the full text content of an indexed file (for agent on another machine to fetch via MCP).
func (s *MCPServer) handleGetDocument(id interface{}, args map[string]interface{}) *MCPResponse {
	filePath, ok := args["file_path"].(string)
	if !ok || filePath == "" {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error: &MCPError{
				Code:    -32602,
				Message: "Missing or invalid 'file_path' parameter",
			},
		}
	}
	if !s.store.HasIndexedFile(filePath) {
		content := []Content{{Type: "text", Text: "File not indexed or path invalid."}}
		return &MCPResponse{JSONRPC: "2.0", ID: id, Result: &ToolsCallResult{Content: content}}
	}
	info, err := os.Stat(filePath)
	if err != nil {
		msg := "file not found"
		if !os.IsNotExist(err) {
			msg = "cannot stat file: " + err.Error()
		}
		content := []Content{{Type: "text", Text: msg}}
		return &MCPResponse{JSONRPC: "2.0", ID: id, Result: &ToolsCallResult{Content: content}}
	}
	if info.Size() > maxDocumentSize {
		content := []Content{{Type: "text", Text: "File too large (max 5MB)."}}
		return &MCPResponse{JSONRPC: "2.0", ID: id, Result: &ToolsCallResult{Content: content}}
	}
	// Use same reader as indexing: .txt as UTF-8, .docx via docx2md to plain text
	text, err := cleaning.ReadFile(filePath)
	if err != nil {
		content := []Content{{Type: "text", Text: "Cannot read file: " + err.Error()}}
		return &MCPResponse{JSONRPC: "2.0", ID: id, Result: &ToolsCallResult{Content: content}}
	}
	if int64(len(text)) > maxDocumentSize {
		content := []Content{{Type: "text", Text: "Extracted text too large (max 5MB)."}}
		return &MCPResponse{JSONRPC: "2.0", ID: id, Result: &ToolsCallResult{Content: content}}
	}
	content := []Content{{Type: "text", Text: text}}
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  &ToolsCallResult{Content: content},
	}
}

// handleGetStats handles the get_stats tool call
func (s *MCPServer) handleGetStats(id interface{}) *MCPResponse {
	stats := s.store.Stats()
	
	statsText := "Dense RAG Statistics:\n"
	statsText += fmt.Sprintf("- Indexed Files: %d\n", stats.IndexedFiles)
	statsText += fmt.Sprintf("- Vector Count: %d\n", stats.VectorCount)
	statsText += fmt.Sprintf("- Store Size: %d bytes\n", stats.StoreSizeBytes)
	
	content := []Content{
		{
			Type: "text",
			Text: statsText,
		},
	}
	
	result := ToolsCallResult{Content: content}
	
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}