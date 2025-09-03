package main

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/typicalfo/forge/backend/internal/db"
	"github.com/typicalfo/forge/backend/internal/handlers"
	"github.com/typicalfo/forge/backend/internal/logging"
	"github.com/typicalfo/forge/backend/internal/mcp"
	"github.com/typicalfo/forge/backend/internal/services"
)

func main() {
	// Initialize SQLite-backed config and seed defaults
	boot, err := initConfig()
	if err != nil {
		logging.GetLogger().WithError(err).Fatal("Failed to init config store")
	}
	defer boot.ConfigStore.Close()
	vals, err := boot.ConfigStore.GetAll()
	if err != nil {
		logging.GetLogger().WithError(err).Fatal("Failed to read config values")
	}
	basePath := vals.ChromaURL
	mcpPort := "8081" // MCP is stdio; port unused but kept for compatibility

	// Initialize Chroma DB
	chromaDB, err := db.NewChromaDB(basePath)
	if err != nil {
		logging.GetLogger().WithError(err).Fatal("Failed to initialize Chroma DB")
	}
	defer func() {
		if err := chromaDB.Close(); err != nil {
			logging.GetLogger().WithError(err).Warn("Error closing ChromaDB client")
		}
	}()

	// Sanity check: Heartbeat
	if err := chromaDB.Health(context.Background()); err != nil {
		logging.GetLogger().WithError(err).Fatal("ChromaDB health check failed")
	}
	logging.GetLogger().Info("ChromaDB is healthy")

	// Initialize services (without collection - collections will be handled per request)
	ingestService := services.NewIngestService(chromaDB.Client())

	// Initialize handlers
	apiHandlers := handlers.NewAPIHandlers(ingestService)

	// Initialize Gin router
	r := gin.Default()
	// Inject config store into handlers for /config endpoint
	apiHandlers = apiHandlers.WithConfigStore(boot.ConfigStore)

	// Add CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Routes
	r.GET("/health", apiHandlers.Health)
	r.GET("/config", apiHandlers.Config)
	// Canonical endpoints
	r.POST("/collections", apiHandlers.CreateCollection)
	r.GET("/collections", apiHandlers.ListCollections)
	r.DELETE("/collections/:name", apiHandlers.DeleteCollection)

	r.GET("/docs/:collection", apiHandlers.GetCollectionDocuments)
	r.DELETE("/docs/:collection/:id", apiHandlers.DeleteDoc)

	r.POST("/search", apiHandlers.Search)

	// Unified ingestion endpoint (handles both file uploads and direct text input)
	r.POST("/api/ingest", apiHandlers.Ingest)

	// Initialize MCP server (without collection - will handle collections dynamically)
	mcpServer := mcp.NewMCPServer(chromaDB.Client())
	go mcpServer.Start(mcpPort)

	// Graceful shutdown handling
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	addr := ":8080"
	if vals.BackendHTTPPort > 0 {
		addr = fmt.Sprintf(":%d", vals.BackendHTTPPort)
	}
	server := &http.Server{Addr: addr, Handler: r}

	go func() {
		logging.GetLogger().Infof("Starting backend server on %s...", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.GetLogger().WithError(err).Error("Server error")
			return
		}
	}()

	<-ctx.Done()
	logging.GetLogger().Info("Shutting down backend...")
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctxShutdown); err != nil {
		logging.GetLogger().WithError(err).Error("Server shutdown error")
	}
	logging.GetLogger().Info("Backend shutdown complete")
}
