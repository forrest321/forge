# Backend Plan: MCP search + multi-file ingest with MD5 dedupe

## 1) Add MD5 dedupe in ingest
- File: internal/services/ingest.go
- Changes:
  - Compute MD5 of full file bytes prior to chunking.
  - Query Chroma collection for existing documents where metadata.file_md5 == md5Hex. If found, skip ingestion and return a typed "already ingested" result.
  - On new ingestion: include metadata on every chunk: file_md5 (string), file_name (string), timestamp (Unix seconds), chunk_index (int). Keep embeddings nil to force server-side embedding by Chroma.
- Use Context7 MCP to fetch chroma-go docs for metadata filters (where clauses) and batching best practices.

## 2) Update HTTP handler to accept multiple files + metadata
- File: internal/handlers/api.go
- Changes:
  - Use c.MultipartForm() and read multiple files from field "files".
  - Optionally read "metadata" (JSON) and merge with per-chunk metadata.
  - For each file: io.ReadAll, compute MD5, call IngestFile, collect per-file status (ingested/skipped).
  - Return JSON summary with per-file results. Add basic per-file size guard (configurable).

## 3) Implement proper MCP server with search tool
- File: internal/mcp/server.go (replace placeholder)
- Implement a real MCP server exposing tools:
  - search: input { query: string, k?: number, filter?: object }, returns top-k results from Chroma (ids, documents, metadatas, distances).
  - health: optional tool to verify Chroma connectivity.
- Entry integration: keep backend/cmd/main.go launcher and start MCP server on a goroutine with graceful shutdown.
- Use Context7 MCP to fetch MCP Go SDK docs: tool registration, schemas, server lifecycle.

## 4) Add search HTTP endpoint for frontend
- File: internal/handlers/api.go
- Route: POST /api/search
  - Input JSON { query: string, k?: number, filter?: object }
  - Call service that wraps db.Query and return structured results.

## 5) Configuration and sanity checks
- File: cmd/main.go
- On startup: Heartbeat Chroma (existing). Optionally dev-only sanity query.
- Env: CHROMA_URL (default http://localhost:8000), CHROMA_COLLECTION (default dev_tool_collection), MCP_PORT (default 8081).

## 6) Tests (Go)
- File: internal/services/ingest_test.go
  - Table-driven tests: MD5 dedupe (mock Query returns hit), new ingestion (Add called with nil embeddings + expected metadata keys). Use context timeouts.
- File: internal/handlers/api_test.go
  - HTTP tests for /api/ingest (multi-file) and /api/search.

## 7) Cleanup
- File: cmd/forge/main.go
  - Remove or deprecate to avoid duplicate entrypoints.

## 8) Build/test
- Commands:
  - make -C backend tidy
  - make -C backend lint (if golangci-lint installed)
  - make -C backend test
  - make -C backend build
