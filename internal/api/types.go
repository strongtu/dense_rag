package api

// QueryRequest is the JSON body for the /query endpoint.
type QueryRequest struct {
	Text string `json:"text"`
}

// ResultItem represents a single search result.
type ResultItem struct {
	Text     string  `json:"text"`
	FilePath string  `json:"file_path"`
	Score    float32 `json:"score"`
}

// QueryResponse is a list of search results.
type QueryResponse []ResultItem

// HealthResponse is the JSON body for the /health endpoint.
type HealthResponse struct {
	Status         string `json:"status"`
	VectorCount    int    `json:"vector_count"`
	IndexedFiles   int    `json:"indexed_files"`
	StoreSizeBytes int64  `json:"store_size_bytes"`
}
