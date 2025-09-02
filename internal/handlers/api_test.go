package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/typicalfo/forge/backend/internal/db"
	"github.com/typicalfo/forge/backend/internal/services"
)

func TestAPIHandlers_Ingest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	chromaDB, err := db.NewChromaDB("http://localhost:8000")
	if err != nil {
		t.Skip("ChromaDB not available")
	}

	_, err = chromaDB.EnsureCollection(context.Background(), "test_collection")
	if err != nil {
		t.Skip("Cannot create collection")
	}

	ingestService := services.NewIngestService(chromaDB.Client())
	handlers := NewAPIHandlers(ingestService)

	router := gin.Default()
	router.POST("/api/ingest", handlers.Ingest)

	t.Run("single file", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, _ := writer.CreateFormFile("files", "test.txt")
		part.Write([]byte("Test content"))

		writer.Close()

		req, _ := http.NewRequest("POST", "/api/ingest", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		t.Logf("Response: %s", w.Body.String())
	})
}

func TestAPIHandlers_Search(t *testing.T) {
	gin.SetMode(gin.TestMode)

	chromaDB, err := db.NewChromaDB("http://localhost:8000")
	if err != nil {
		t.Skip("ChromaDB not available")
	}

	_, err = chromaDB.EnsureCollection(context.Background(), "test_collection")
	if err != nil {
		t.Skip("Cannot create collection")
	}

	ingestService := services.NewIngestService(chromaDB.Client())
	handlers := NewAPIHandlers(ingestService)

	router := gin.Default()
	router.POST("/api/search", handlers.Search)

	reqBody := map[string]interface{}{
		"query":         "test",
		"collection_id": "test_collection",
		"k":             5,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "/api/search", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	t.Logf("Response: %s", w.Body.String())
}
