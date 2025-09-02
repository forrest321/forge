package services

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	chroma "github.com/forrest321/chroma-go/pkg/api/v2"
	"github.com/sirupsen/logrus"
	"github.com/typicalfo/forge/backend/internal/logging"
)

type IngestResult struct {
	Status string `json:"status"` // "ingested" or "skipped"
	File   string `json:"file"`
	Chunks int    `json:"chunks,omitempty"`
}

type IngestService struct {
	chromaDB chroma.Client
}

func NewIngestService(chromaDB chroma.Client) *IngestService {
	return &IngestService{chromaDB: chromaDB}
}

func (s *IngestService) IngestFile(ctx context.Context, collectionName string, filePath string, content []byte, userMetadata map[string]interface{}) (*IngestResult, error) {
	// Get or create collection
	collection, err := s.chromaDB.GetOrCreateCollection(ctx, collectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create collection: %w", err)
	}
	// Compute MD5 of file content for dedupe
	md5Hash := fmt.Sprintf("%x", md5.Sum(content))

	// Check if file already ingested by querying for existing MD5
	results, err := collection.Get(ctx, chroma.WithWhereGet(chroma.EqString("file_md5", md5Hash)))
	if err != nil {
		logging.GetLogger().WithError(err).WithField("file", filePath).Error("Error querying for dedupe")
		return nil, err
	}

	// Check if we got any results
	docs := results.GetDocuments()
	if len(docs) > 0 {
		logging.GetLogger().WithFields(logrus.Fields{
			"file": filePath,
			"md5":  md5Hash,
		}).Info("File already ingested, skipping")
		return &IngestResult{Status: "skipped", File: filePath}, nil
	}

	// Extract text (assume text-based files)
	text := string(content)

	// Chunk into segments (simple: split by lines, limit to 512 tokens approx)
	chunks := chunkText(text, 512)

	// Generate IDs and metadata
	ids := make([]string, len(chunks))
	metadatas := make([]map[string]interface{}, len(chunks))
	for i, chunk := range chunks {
		hash := sha256.Sum256([]byte(filePath + chunk + fmt.Sprintf("%d", i)))
		ids[i] = fmt.Sprintf("%x", hash[:8])

		// Start with system metadata
		metadata := map[string]interface{}{
			"file_md5":    md5Hash,
			"file_name":   filePath,
			"timestamp":   time.Now().Unix(),
			"chunk_index": i,
		}

		// Merge user metadata if provided
		if userMetadata != nil {
			for key, value := range userMetadata {
				// Prefix user metadata keys to avoid conflicts with system metadata
				metadata["user_"+key] = value
			}
		}

		metadatas[i] = metadata
	}

	// Convert metadatas to chroma format
	var chromaMetadatas []chroma.DocumentMetadata
	for _, m := range metadatas {
		var attributes []*chroma.MetaAttribute
		for k, v := range m {
			switch val := v.(type) {
			case string:
				attributes = append(attributes, chroma.NewStringAttribute(k, val))
			case int64:
				attributes = append(attributes, chroma.NewIntAttribute(k, val))
			case float64:
				attributes = append(attributes, chroma.NewFloatAttribute(k, val))
			}
		}
		metadata := chroma.NewDocumentMetadata(attributes...)
		chromaMetadatas = append(chromaMetadatas, metadata)
	}

	// Convert IDs to DocumentIDs
	var docIDs chroma.DocumentIDs
	for _, id := range ids {
		docIDs = append(docIDs, chroma.DocumentID(id))
	}

	// Add to collection
	err = collection.Add(ctx,
		chroma.WithIDs(docIDs...),
		chroma.WithTexts(chunks...),
		chroma.WithMetadatas(chromaMetadatas...))
	if err != nil {
		logging.GetLogger().WithError(err).WithField("file", filePath).Error("Error adding to collection")
		return nil, err
	}

	logging.GetLogger().WithFields(logrus.Fields{
		"file":   filePath,
		"chunks": len(chunks),
	}).Info("Successfully ingested file")
	return &IngestResult{Status: "ingested", File: filePath, Chunks: len(chunks)}, nil
}

type SearchResult struct {
	ID       string                 `json:"id"`
	Document string                 `json:"document"`
	Metadata map[string]interface{} `json:"metadata"`
	Distance float32                `json:"distance"`
}

func (s *IngestService) Search(ctx context.Context, collectionName string, query string, k int, filter map[string]interface{}) ([]SearchResult, error) {
	// Try to get collection first
	collection, err := s.chromaDB.GetCollection(ctx, collectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection '%s': %w", collectionName, err)
	}

	var queryOptions []chroma.CollectionQueryOption
	queryOptions = append(queryOptions, chroma.WithQueryTexts(query))
	queryOptions = append(queryOptions, chroma.WithNResults(k))

	// Add filter if provided
	if len(filter) > 0 {
		var whereClause chroma.WhereFilter
		for k, v := range filter {
			switch val := v.(type) {
			case string:
				whereClause = chroma.EqString(k, val)
			case int:
				whereClause = chroma.EqInt(k, val)
			case float64:
				whereClause = chroma.EqFloat(k, float32(val))
			}
		}
		queryOptions = append(queryOptions, chroma.WithWhereQuery(whereClause))
	}

	results, err := collection.Query(ctx, queryOptions...)
	if err != nil {
		logging.GetLogger().WithError(err).WithField("queryOptions", queryOptions).Error("Error querying collection")
		return nil, err
	}

	var searchResults []SearchResult

	// QueryResult returns groups - we want the first group
	docsGroups := results.GetDocumentsGroups()
	idsGroups := results.GetIDGroups()
	metadatasGroups := results.GetMetadatasGroups()
	distancesGroups := results.GetDistancesGroups()

	if len(docsGroups) > 0 && len(docsGroups[0]) > 0 {
		docs := docsGroups[0]
		ids := idsGroups[0]
		metadatas := metadatasGroups[0]
		distances := distancesGroups[0]

		for i, doc := range docs {
			// Convert metadata back to map
			metadataMap := make(map[string]interface{})
			if i < len(metadatas) && metadatas[i] != nil {
				// This is a simplified conversion - you might need to handle different attribute types
				metadataMap = map[string]interface{}{
					"id":       string(ids[i]),
					"document": doc,
				}
				if i < len(distances) {
					metadataMap["distance"] = distances[i]
				}
			}

			searchResults = append(searchResults, SearchResult{
				ID:       string(ids[i]),
				Document: doc.ContentString(),
				Metadata: metadataMap,
				Distance: float32(distances[i]),
			})
		}
	}

	return searchResults, nil
}

func chunkText(text string, maxTokens int) []string {
	lines := strings.Split(text, "\n")
	var chunks []string
	var currentChunk strings.Builder
	tokenCount := 0

	for _, line := range lines {
		lineTokens := len(strings.Fields(line))
		if tokenCount+lineTokens > maxTokens {
			if currentChunk.Len() > 0 {
				chunks = append(chunks, currentChunk.String())
				currentChunk.Reset()
				tokenCount = 0
			}
		}
		currentChunk.WriteString(line + "\n")
		tokenCount += lineTokens
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, currentChunk.String())
	}

	return chunks
}

func (s *IngestService) ListCollections(ctx context.Context) ([]string, error) {
	collections, err := s.chromaDB.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	var names []string
	for _, collection := range collections {
		names = append(names, collection.Name())
	}

	return names, nil
}

func (s *IngestService) CreateCollection(ctx context.Context, name string, description string) (map[string]interface{}, error) {
	collection, err := s.chromaDB.GetOrCreateCollection(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to create collection: %w", err)
	}

	return map[string]interface{}{
		"id":          collection.ID(),
		"name":        collection.Name(),
		"description": description,
	}, nil
}

type Document struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata"`
	FilePath  string                 `json:"file_path,omitempty"`
	CreatedAt string                 `json:"created_at,omitempty"`
}

// CreateDocDirect creates a single document directly without chunking or deduplication
func (s *IngestService) CreateDocDirect(ctx context.Context, collectionName, id, text string, metadata map[string]interface{}) (string, error) {
	if text == "" {
		return "", fmt.Errorf("text is required")
	}
	collection, err := s.chromaDB.GetOrCreateCollection(ctx, collectionName)
	if err != nil {
		return "", fmt.Errorf("get/create collection: %w", err)
	}
	// Determine ID
	docID := id
	if docID == "" {
		h := sha256.Sum256([]byte(text))
		docID = fmt.Sprintf("%x", h[:8])
	}
	// If id provided, check conflict
	if id != "" {
		got, err := collection.Get(ctx, chroma.WithIDsGet(chroma.DocumentID(id)))
		if err == nil && len(got.GetIDs()) > 0 {
			return "", fmt.Errorf("conflict: id already exists")
		}
	}
	// Build metadata
	var attrs []*chroma.MetaAttribute
	for k, v := range metadata {
		switch val := v.(type) {
		case string:
			attrs = append(attrs, chroma.NewStringAttribute(k, val))
		case int:
			attrs = append(attrs, chroma.NewIntAttribute(k, int64(val)))
		case int64:
			attrs = append(attrs, chroma.NewIntAttribute(k, val))
		case float64:
			attrs = append(attrs, chroma.NewFloatAttribute(k, val))
		}
	}
	md := chroma.NewDocumentMetadata(attrs...)
	// Add
	err = collection.Add(ctx,
		chroma.WithIDs(chroma.DocumentID(docID)),
		chroma.WithTexts(text),
		chroma.WithMetadatas(md),
	)
	if err != nil {
		return "", fmt.Errorf("add document: %w", err)
	}
	return docID, nil
}

// DeleteDoc deletes a document by id from a collection
func (s *IngestService) DeleteDoc(ctx context.Context, collectionName, id string) error {
	collection, err := s.chromaDB.GetCollection(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("err getting collection %s to delete: %w", collectionName, err)

	}
	return collection.Delete(ctx, chroma.WithIDsDelete(chroma.DocumentID(id)))
}

// DeleteCollection removes the entire collection
func (s *IngestService) DeleteCollection(ctx context.Context, name string) error {
	return s.chromaDB.DeleteCollection(ctx, name)
}

func (s *IngestService) GetCollectionDocuments(ctx context.Context, collectionName string) ([]Document, error) {
	logging.GetLogger().WithField("collectionName", collectionName).Info("Getting collection documents")

	// Get the collection
	collection, err := s.chromaDB.GetCollection(ctx, collectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection: %w", err)
	}

	logging.GetLogger().WithField("collectionName", collectionName).Info("Collection found, getting documents")

	// Get all documents from the collection
	results, err := collection.Get(ctx)
	if err != nil {
		logging.GetLogger().WithError(err).WithField("collectionName", collectionName).Error("Failed to get documents from collection")
		return nil, fmt.Errorf("failed to get documents: %w", err)
	}

	logging.GetLogger().WithFields(logrus.Fields{
		"collectionName": collectionName,
		"documentCount":  len(results.GetDocuments()),
	}).Info("Retrieved documents from collection")

	var documents []Document

	// Extract document data
	docs := results.GetDocuments()
	ids := results.GetIDs()
	metadatas := results.GetMetadatas()

	for i, doc := range docs {
		document := Document{
			ID:       string(ids[i]),
			Content:  doc.ContentString(),
			Metadata: make(map[string]interface{}),
		}

		// Add basic metadata
		document.Metadata["id"] = string(ids[i])

		// Add metadata if available
		if i < len(metadatas) && metadatas[i] != nil {
			// Create a new metadata map with basic info
			document.Metadata["document"] = doc.ContentString()
		}

		documents = append(documents, document)
	}

	return documents, nil
}
