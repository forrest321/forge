# Forge Backend

A Go-based backend service for the Forge project, providing file ingestion into ChromaDB and MCP server integration.

## Overview

The backend serves static frontend files, exposes APIs for file ingestion, and integrates with ChromaDB for vector storage. It also includes an MCP server for tool-based interactions.

## Prerequisites

- Go 1.20 or later
- ChromaDB running locally (via Docker or otherwise)
- Optional: OpenAI API key for embeddings

## Setup

1. Clone the repository and navigate to the backend directory:
   ```bash
   cd backend
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Ensure ChromaDB is running:
   ```bash
   # Example with Docker
   docker run -p 8000:8000 chromadb/chroma
   ```

## Configuration

Set environment variables as needed:

- `CHROMA_URL`: ChromaDB server URL (default: `http://localhost:8000`)
- `CHROMA_COLLECTION`: Collection name (default: `dev_tool_collection`)
- `MCP_PORT`: MCP server port (default: `8081`)
- `OPENAI_API_KEY`: For OpenAI embeddings (optional)

## Build and Run

### Using Make
```bash
make build    # Build the backend
make run      # Run the backend
make test     # Run tests
make lint     # Lint the code
make clean    # Clean build artifacts
```

### Manual Commands
```bash
go build ./...          # Build
go run ./cmd            # Run
go test ./...           # Test
golangci-lint run       # Lint (if installed)
```

## API Endpoints

- `GET /health`: Health check
- `POST /api/ingest`: Ingest a file (multipart/form-data with `file` field)

### Example Usage
```bash
# Health check
curl http://localhost:8080/health

# Ingest a file
curl -F "file=@example.txt" http://localhost:8080/api/ingest
```

## MCP Server

The backend includes an MCP server placeholder running on port 8081. It registers a search tool for querying the Chroma collection.

## Development

- Code style: Follow Go conventions, use `gofmt` and `goimports`
- Tests: Add unit tests in `*_test.go` files
- Linting: Use `golangci-lint` for code quality

## Project Structure

```
backend/
├── cmd/
│   └── main.go          # Main entry point
├── internal/
│   ├── db/
│   │   └── chroma.go    # ChromaDB client wrapper
│   ├── services/
│   │   └── ingest.go    # Ingestion service
│   ├── handlers/
│   │   └── api.go       # HTTP handlers
│   └── mcp/
│       └── server.go    # MCP server
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## Contributing

1. Follow the code style guidelines in AGENTS.md
2. Add tests for new features
3. Ensure `make build` and `make test` pass

## License

[Add license information if applicable]
