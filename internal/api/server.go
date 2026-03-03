package api

import (
	"context"
	"fmt"
	"net/http"

	"dense-rag/internal/config"
	"dense-rag/internal/embedding"
	"dense-rag/internal/mcp"
	"dense-rag/internal/store"

	"github.com/gin-gonic/gin"
)

// Server wraps the HTTP server and its dependencies.
type Server struct {
	engine      *gin.Engine
	httpServer  *http.Server
	store       *store.Store
	embedClient *embedding.Client
	config      *config.Config
	mcpServer   *mcp.MCPServer
}

// NewServer creates and configures a new API server.
func NewServer(cfg *config.Config, st *store.Store, embedClient *embedding.Client) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	s := &Server{
		engine:      engine,
		store:       st,
		embedClient: embedClient,
		config:      cfg,
		mcpServer:   mcp.NewMCPServer(st, embedClient, cfg.TopK),
	}

	s.registerRoutes()
	return s
}

// registerRoutes wires handler functions to their endpoints.
func (s *Server) registerRoutes() {
	s.engine.POST("/query", handleQuery(s.embedClient, s.store, s.config.TopK))
	s.engine.GET("/health", handleHealth(s.embedClient, s.store))
	s.engine.POST("/mcp", handleMCP(s.mcpServer))
}

// Start begins listening on the configured host and port. It blocks until
// the server shuts down or encounters an error.
func (s *Server) Start(ctx context.Context) error {
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.config.Host, s.config.Port),
		Handler: s.engine,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		return s.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

// Engine returns the underlying gin.Engine for testing purposes.
func (s *Server) Engine() *gin.Engine {
	return s.engine
}
