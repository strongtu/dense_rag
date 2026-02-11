package api

import (
	"net/http"

	"dense-rag/internal/embedding"
	"dense-rag/internal/store"

	"github.com/gin-gonic/gin"
)

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
