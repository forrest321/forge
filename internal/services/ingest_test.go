package services

import (
	"context"
	"testing"

	"github.com/typicalfo/forge/backend/internal/db"
)

func TestIngestService_IngestFile(t *testing.T) {
	// Note: This is a basic test; in real scenario, use a test Chroma instance or mock
	// For now, assume client and collection are set up

	chromaDB, err := db.NewChromaDB("http://localhost:8000")
	if err != nil {
		t.Skip("ChromaDB not available for testing")
	}

	_, err = chromaDB.EnsureCollection(context.Background(), "test_collection")
	if err != nil {
		t.Skip("Cannot create test collection")
	}

	service := NewIngestService(chromaDB.Client())

	tests := []struct {
		name         string
		filePath     string
		content      []byte
		userMetadata map[string]interface{}
		wantErr      bool
	}{
		{
			name:         "new file",
			filePath:     "test.txt",
			content:      []byte("This is a test document."),
			userMetadata: nil,
			wantErr:      false,
		},
		{
			name:         "file with metadata",
			filePath:     "test2.txt",
			content:      []byte("This is another test document."),
			userMetadata: map[string]interface{}{"category": "test", "author": "user"},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.IngestFile(context.Background(), "test_collection", tt.filePath, tt.content, tt.userMetadata)
			if (err != nil) != tt.wantErr {
				t.Errorf("IngestFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != nil {
				t.Logf("Result: %+v", result)
			}
		})
	}
}

func TestIngestService_Search(t *testing.T) {
	chromaDB, err := db.NewChromaDB("http://localhost:8000")
	if err != nil {
		t.Skip("ChromaDB not available for testing")
	}

	_, err = chromaDB.EnsureCollection(context.Background(), "test_collection")
	if err != nil {
		t.Skip("Cannot create test collection")
	}

	service := NewIngestService(chromaDB.Client())

	results, err := service.Search(context.Background(), "test_collection", "test", 5, nil)
	if err != nil {
		t.Errorf("Search() error = %v", err)
		return
	}
	t.Logf("Search results: %+v", results)
}
