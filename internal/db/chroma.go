package db

import (
	"context"
	"fmt"
	"log"

	chroma "github.com/forrest321/chroma-go"
	defaultef "github.com/forrest321/chroma-go/pkg/embeddings/default_ef"
)

type ChromaDB struct {
	client chroma.Client
}

func NewChromaDB(basePath string) (*ChromaDB, error) {
	client, err := chroma.NewHTTPClient(chroma.WithBaseURL(basePath))
	if err != nil {
		return nil, err
	}
	return &ChromaDB{client: client}, nil
}

func (db *ChromaDB) Client() chroma.Client {
	return db.client
}

func (db *ChromaDB) Health(ctx context.Context) error {
	return db.client.Heartbeat(ctx)
}

func (db *ChromaDB) GetOrCreateCollection(ctx context.Context, name string) (chroma.Collection, error) {
	// Try to get existing collection first
	collection, err := db.client.GetCollection(ctx, name)
	if err == nil {
		return collection, nil
	}

	// If collection doesn't exist, create it with default settings
	return db.CreateCollection(ctx, name, "")
}

func (db *ChromaDB) CreateCollection(ctx context.Context, name, description string) (chroma.Collection, error) {
	ef, closeef, efErr := defaultef.NewDefaultEmbeddingFunction()

	// make sure to call this to ensure proper resource release
	defer func() {
		err := closeef()
		if err != nil {
			fmt.Printf("Error closing default embedding function: %s \n", err)
		}
	}()
	if efErr != nil {
		fmt.Printf("Error creating OpenAI embedding function: %s \n", efErr)
	}

	col, err := db.client.GetOrCreateCollection(context.Background(), name,
		chroma.WithCollectionMetadataCreate(
			chroma.NewMetadata(
				chroma.NewStringAttribute("description", description),
			),
		),
		chroma.WithEmbeddingFunctionCreate(ef),
	)
	if err != nil {
		log.Fatalf("Error creating collection: %s \n", err)
		return nil, err
	}
	return col, nil
}

func (db *ChromaDB) DeleteCollection(ctx context.Context, name string) error {
	return db.client.DeleteCollection(ctx, name)
}

func (db *ChromaDB) RecreateCollection(ctx context.Context, name string) (chroma.Collection, error) {
	// Delete existing collection if it exists
	_ = db.client.DeleteCollection(ctx, name)

	// Create new collection
	return db.client.CreateCollection(ctx, name)
}

func (db *ChromaDB) AddDocuments(ctx context.Context, collection chroma.Collection, documents []string, metadatas []map[string]interface{}, ids []string) error {
	// Convert metadatas to the new format
	var chromaMetadatas []chroma.DocumentMetadata
	for _, m := range metadatas {
		var attributes []*chroma.MetaAttribute
		for k, v := range m {
			switch val := v.(type) {
			case string:
				attributes = append(attributes, chroma.NewStringAttribute(k, val))
			case int:
				attributes = append(attributes, chroma.NewIntAttribute(k, int64(val)))
			case float64:
				attributes = append(attributes, chroma.NewFloatAttribute(k, val))
			}
		}
		metadata := chroma.NewDocumentMetadata(attributes...)
		chromaMetadatas = append(chromaMetadatas, metadata)
	}

	// Convert string IDs to DocumentIDs
	var docIDs chroma.DocumentIDs
	for _, id := range ids {
		docIDs = append(docIDs, chroma.DocumentID(id))
	}

	return collection.Add(ctx,
		chroma.WithIDs(docIDs...),
		chroma.WithTexts(documents...),
		chroma.WithMetadatas(chromaMetadatas...))
}

func (db *ChromaDB) Query(ctx context.Context, collection chroma.Collection, queryTexts []string, nResults int) (chroma.QueryResult, error) {
	results, err := collection.Query(ctx,
		chroma.WithQueryTexts(queryTexts...),
		chroma.WithNResults(nResults))
	if err != nil {
		return nil, err
	}
	return results, nil
}
