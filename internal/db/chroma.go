package db

import (
	"context"
	"fmt"

	chroma "github.com/forrest321/chroma-go/pkg/api/v2"
)

type ChromaDB struct {
	client chroma.Client
}

// NewChromaDB creates a minimal HTTP client. YAGNI: only base URL support.
func NewChromaDB(baseURL string) (*ChromaDB, error) {
	var opts []chroma.ClientOption
	if baseURL != "" {
		opts = append(opts, chroma.WithBaseURL(baseURL))
	}
	client, err := chroma.NewHTTPClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("create chroma http client: %w", err)
	}
	return &ChromaDB{client: client}, nil
}

// Close releases underlying resources (e.g., local embedding functions).
func (c *ChromaDB) Close() error { return c.client.Close() }

// Client exposes the underlying chroma client for services.
func (c *ChromaDB) Client() chroma.Client { return c.client }

// Health performs a lightweight heartbeat against the Chroma server.
func (c *ChromaDB) Health(ctx context.Context) error { return c.client.Heartbeat(ctx) }

// EnsureCollection returns an existing collection or creates it if missing.
func (c *ChromaDB) EnsureCollection(ctx context.Context, name string) (chroma.Collection, error) {
	col, err := c.client.GetOrCreateCollection(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("ensure collection %q: %w", name, err)
	}
	return col, nil
}

// GetCollection returns an existing collection by name.
func (c *ChromaDB) GetCollection(ctx context.Context, name string) (chroma.Collection, error) {
	col, err := c.client.GetCollection(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get collection %q: %w", name, err)
	}
	return col, nil
}

// ListCollections returns all collections.
func (c *ChromaDB) ListCollections(ctx context.Context) ([]chroma.Collection, error) {
	cols, err := c.client.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}
	return cols, nil
}

// DeleteCollection removes a collection by name.
func (c *ChromaDB) DeleteCollection(ctx context.Context, name string) error {
	if err := c.client.DeleteCollection(ctx, name); err != nil {
		return fmt.Errorf("delete collection %q: %w", name, err)
	}
	return nil
}

// ListDocuments returns all documents in a collection.
func (c *ChromaDB) ListDocuments(ctx context.Context, collectionName string) (chroma.GetResult, error) {
	col, err := c.GetCollection(ctx, collectionName)
	if err != nil {
		return nil, err
	}
	res, err := col.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("get documents for %q: %w", collectionName, err)
	}
	return res, nil
}

// Search searches documents in a collection with optional k and simple equality filter.
func (c *ChromaDB) Search(ctx context.Context, collectionName, query string, k int, filter map[string]interface{}) (chroma.QueryResult, error) {
	col, err := c.GetCollection(ctx, collectionName)
	if err != nil {
		return nil, err
	}
	var opts []chroma.CollectionQueryOption
	opts = append(opts, chroma.WithQueryTexts(query))
	if k > 0 {
		opts = append(opts, chroma.WithNResults(k))
	}
	// Minimal where support: single-key equality like in services.
	if len(filter) > 0 {
		var where chroma.WhereFilter
		for key, v := range filter { // take last entry if multiple, YAGNI
			switch val := v.(type) {
			case string:
				where = chroma.EqString(key, val)
			case int:
				where = chroma.EqInt(key, val)
			case float64:
				where = chroma.EqFloat(key, float32(val))
			}
		}
		if where != nil {
			opts = append(opts, chroma.WithWhereQuery(where))
		}
	}
	res, err := col.Query(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("query collection %q: %w", collectionName, err)
	}
	return res, nil
}

// DeleteDocument deletes a single document by ID in a collection.
func (c *ChromaDB) DeleteDocument(ctx context.Context, collectionName, id string) error {
	col, err := c.GetCollection(ctx, collectionName)
	if err != nil {
		return err
	}
	if err := col.Delete(ctx, chroma.WithIDsDelete(chroma.DocumentID(id))); err != nil {
		return fmt.Errorf("delete doc %q in %q: %w", id, collectionName, err)
	}
	return nil
}
