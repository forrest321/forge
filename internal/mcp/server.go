package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	chroma "github.com/forrest321/chroma-go/pkg/api/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/typicalfo/forge/backend/internal/logging"
	"github.com/typicalfo/forge/backend/internal/services"
)

// MCPServer is a minimal MCP server wrapper.
// YAGNI: Just what we need to expose search + health.
type MCPServer struct {
	chromaDB chroma.Client
}

func NewMCPServer(chromaDB chroma.Client) *MCPServer {
	return &MCPServer{chromaDB: chromaDB}
}

// Start runs the MCP server until the provided context is canceled.
func (s *MCPServer) Start(ctx context.Context, port string) {
	logging.GetLogger().WithField("port", port).Info("Starting MCP server")

	server := mcp.NewServer(&mcp.Implementation{Name: "Forge MCP Server", Version: "1.0.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{Name: "search", Description: "Search the ingested documents using semantic similarity"}, s.handleSearchFunc())
	mcp.AddTool(server, &mcp.Tool{Name: "health", Description: "Check the health of the ChromaDB connection"}, s.handleHealthFunc())

	logging.GetLogger().Info("MCP server ready")
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		logging.GetLogger().WithError(err).Error("MCP server error")
	}
	logging.GetLogger().Info("MCP server stopped")
}

// handleSearchFunc creates a standalone function that can be used with AddTool
func (s *MCPServer) handleSearchFunc() func(context.Context, *mcp.CallToolRequest, SearchParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, args SearchParams) (*mcp.CallToolResult, any, error) {
		k := args.K
		if k == 0 {
			k = 5
		}
		service := services.NewIngestService(s.chromaDB)
		results, err := service.Search(ctx, args.CollectionId, args.Query, k, args.Filter)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Search error: %v", err)}},
			}, nil, nil
		}
		resultJSON, _ := json.Marshal(results)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(resultJSON)}},
		}, nil, nil
	}
}

// handleHealthFunc creates a standalone function that can be used with AddTool
func (s *MCPServer) handleHealthFunc() func(context.Context, *mcp.CallToolRequest, HealthParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, args HealthParams) (*mcp.CallToolResult, any, error) {
		if err := s.chromaDB.Heartbeat(ctx); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Health check failed: %v", err)}},
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "ChromaDB is healthy"}},
		}, nil, nil
	}
}

type SearchParams struct {
	Query        string                 `json:"query" jsonschema:"the search query to find similar documents"`
	CollectionId string                 `json:"collection_id" jsonschema:"the collection to search in"`
	K            int                    `json:"k,omitempty" jsonschema:"number of results to return (default: 5)"`
	Filter       map[string]interface{} `json:"filter,omitempty" jsonschema:"optional metadata filter for search results"`
}

type HealthParams struct{}
