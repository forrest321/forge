package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/typicalfo/forge/backend/internal/logging"
	"github.com/typicalfo/forge/backend/internal/services"
)

type APIHandlers struct {
	ingestService *services.IngestService
	configStore   ConfigProvider
}

func NewAPIHandlers(ingestService *services.IngestService) *APIHandlers {
	return &APIHandlers{ingestService: ingestService}
}

func (h *APIHandlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Config returns runtime configuration for the local app (no .env usage)
func (h *APIHandlers) Config(c *gin.Context) {
	if h.configStore == nil {
		c.JSON(http.StatusOK, gin.H{
			"backend_http_port":  8080,
			"chroma_url":         "http://localhost:8000",
			"default_collection": "default",
			"mcp_transport":      "stdio",
		})
		return
	}
	vals, err := h.configStore.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"backend_http_port":  vals.BackendHTTPPort,
		"chroma_url":         vals.ChromaURL,
		"default_collection": vals.CollectionName,
		"mcp_transport":      vals.MCPTransport,
	})
}

func (h *APIHandlers) Ingest(c *gin.Context) {
	// Check if this is a multipart form (file upload) or JSON (direct text input)
	contentType := c.GetHeader("Content-Type")

	if strings.Contains(contentType, "multipart/form-data") {
		// Handle file upload
		h.handleFileUpload(c)
	} else {
		// Handle direct text input
		h.handleDirectText(c)
	}
}

func (h *APIHandlers) handleFileUpload(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer form.RemoveAll()

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no files provided"})
		return
	}

	// Get collection name from form
	collectionName := c.PostForm("collection_id")
	if collectionName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "collection_id is required"})
		return
	}

	// Optional metadata
	var userMetadata map[string]interface{}
	if metadataStr := c.PostForm("metadata"); metadataStr != "" {
		if err := json.Unmarshal([]byte(metadataStr), &userMetadata); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid metadata JSON"})
			return
		}
	}

	var results []services.IngestResult
	for _, fileHeader := range files {
		f, err := fileHeader.Open()
		if err != nil {
			results = append(results, services.IngestResult{Status: "error", File: fileHeader.Filename})
			continue
		}
		defer f.Close()

		buf, err := io.ReadAll(f)
		if err != nil {
			results = append(results, services.IngestResult{Status: "error", File: fileHeader.Filename})
			continue
		}

		// Pass user metadata to the service
		result, err := h.ingestService.IngestFile(c.Request.Context(), collectionName, fileHeader.Filename, buf, userMetadata)
		if err != nil {
			results = append(results, services.IngestResult{Status: "error", File: fileHeader.Filename})
			continue
		}
		results = append(results, *result)
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}

func (h *APIHandlers) handleDirectText(c *gin.Context) {
	var req struct {
		Collection string                 `json:"collection" binding:"required"`
		ID         string                 `json:"id"`
		Text       string                 `json:"text" binding:"required"`
		Metadata   map[string]interface{} `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id, err := h.ingestService.CreateDocDirect(c.Request.Context(), req.Collection, req.ID, req.Text, req.Metadata)
	if err != nil {
		if strings.Contains(err.Error(), "conflict") {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (h *APIHandlers) Search(c *gin.Context) {
	var req struct {
		Query        string                 `json:"query" binding:"required"`
		CollectionId string                 `json:"collection_id" binding:"required"`
		K            int                    `json:"k,omitempty"`
		Filter       map[string]interface{} `json:"filter,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.K == 0 {
		req.K = 5
	}

	// Pass filter to service layer
	results, err := h.ingestService.Search(c.Request.Context(), req.CollectionId, req.Query, req.K, req.Filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}

func (h *APIHandlers) ListCollections(c *gin.Context) {
	collections, err := h.ingestService.ListCollections(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"collections": collections})
}

func (h *APIHandlers) CreateCollection(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	collection, err := h.ingestService.CreateCollection(c.Request.Context(), req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"collection": collection})
}

func (h *APIHandlers) GetCollectionDocuments(c *gin.Context) {
	collectionId := c.Param("collection")
	if collectionId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "collection ID is required"})
		return
	}

	logging.GetLogger().WithField("collectionId", collectionId).Info("Fetching documents for collection")

	documents, err := h.ingestService.GetCollectionDocuments(c.Request.Context(), collectionId)
	if err != nil {
		logging.GetLogger().WithError(err).WithField("collectionId", collectionId).Error("Failed to get collection documents")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	logging.GetLogger().WithFields(logrus.Fields{
		"collectionId":  collectionId,
		"documentCount": len(documents),
	}).Info("Successfully retrieved collection documents")

	c.JSON(http.StatusOK, gin.H{"documents": documents})
}

// New canonical handlers
func (h *APIHandlers) DeleteCollection(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "collection name is required"})
		return
	}
	if err := h.ingestService.DeleteCollection(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *APIHandlers) ListDocs(c *gin.Context) {
	collection := c.Param("collection")
	if collection == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "collection is required"})
		return
	}
	docs, err := h.ingestService.GetCollectionDocuments(c.Request.Context(), collection)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"documents": docs})
}

func (h *APIHandlers) DeleteDoc(c *gin.Context) {
	collection := c.Param("collection")
	id := c.Param("id")
	if collection == "" || id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "collection and id are required"})
		return
	}
	if err := h.ingestService.DeleteDoc(c.Request.Context(), collection, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
