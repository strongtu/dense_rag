package api

import (
	"log"
	"net/http"
	"os"

	"dense-rag/internal/cleaning"
	"dense-rag/internal/embedding"
	"dense-rag/internal/mcp"
	"dense-rag/internal/store"

	"github.com/gin-gonic/gin"
)

const maxDocumentSize = 5 << 20 // 5MB

// handleQuery processes a semantic search request.
func handleQuery(embedClient *embedding.Client, st *store.Store, topK int) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req QueryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		if req.Text == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "text must not be empty"})
			return
		}

		vec, err := embedClient.EmbedSingle(c.Request.Context(), req.Text)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "embedding failed: " + err.Error()})
			return
		}

		results := st.Search(vec, topK)

		resp := make(QueryResponse, len(results))
		for i, r := range results {
			resp[i] = ResultItem{
				Text:     r.Text,
				FilePath: r.FilePath,
				Score:    r.Score,
			}
		}

		c.JSON(http.StatusOK, resp)
	}
}

// handleHealth returns the health status of the service.
func handleHealth(embedClient *embedding.Client, st *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		stats := st.Stats()

		status := "ok"
		if err := embedClient.Ping(c.Request.Context()); err != nil {
			status = "degraded"
		}

		c.JSON(http.StatusOK, HealthResponse{
			Status:         status,
			VectorCount:    stats.VectorCount,
			IndexedFiles:   stats.IndexedFiles,
			StoreSizeBytes: stats.StoreSizeBytes,
		})
	}
}

// handleDocument returns the full text content of an indexed file. The file_path must be exactly as returned by /query so that agent on another machine can fetch content from this server.
func handleDocument(st *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req DocumentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		if req.FilePath == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "file_path is required"})
			return
		}
		if !st.HasIndexedFile(req.FilePath) {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not indexed or path invalid"})
			return
		}
		info, err := os.Stat(req.FilePath)
		if err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot stat file: " + err.Error()})
			return
		}
		if info.Size() > maxDocumentSize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large (max 5MB)"})
			return
		}
		// Use same reader as indexing: .txt as UTF-8, .docx via docx2md to plain text
		content, err := cleaning.ReadFile(req.FilePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot read file: " + err.Error()})
			return
		}
		if int64(len(content)) > maxDocumentSize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "extracted text too large (max 5MB)"})
			return
		}
		c.JSON(http.StatusOK, DocumentResponse{Content: content})
	}
}

// handleMCP handles MCP JSON-RPC over HTTP (POST /mcp). Body is one MCP request, response is one MCP response.
func handleMCP(mcpServer *mcp.MCPServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req mcp.MCPRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid MCP request: " + err.Error()})
			return
		}
		log.Printf("mcp: request from %s method %s", c.ClientIP(), req.Method)
		resp := mcpServer.HandleRequest(c.Request.Context(), &req)
		c.JSON(http.StatusOK, resp)
	}
}
