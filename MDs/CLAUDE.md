# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run the server
go run .

# Run all tests
go test

# Run benchmarks
go test -bench=. -benchmem

# Run a specific benchmark
go test -bench=BenchmarkParallel -benchmem

# Build binary
go build
```

The server starts on port 8080. Set `DOCS_DATABASE_URL` in a `.env` file to use PostgreSQL; otherwise it falls back to `corpus.json`.

## Architecture

GopherSeek is a full-text search engine implementing BM25 relevance scoring with parallel document processing.

**Data flow:**
1. On startup, `dataFetcher.go` loads documents from PostgreSQL (`DOCS_DATABASE_URL`) or `corpus.json`, tokenizes them, and builds an in-memory inverted index.
2. HTTP handlers in `search.go` receive queries via `GET /search?query=<term>`.
3. `getDocumentScoresByIdParallel()` chunks the corpus across `runtime.NumCPU()` goroutines, each computing BM25 scores for its slice.
4. Workers send `[]scoreResult` back through a buffered channel; results are aggregated and sorted by `getTopSearchResults()`.
5. `GET /inverted-index` returns the raw token→docID map for inspection.

**Key files:**
- `search.go` — HTTP server, parallel scoring orchestration, handlers
- `bm25.go` — BM25 algorithm: IDF + TF with k=1.2, b=0.75
- `dataFetcher.go` — corpus loading (DB or file), tokenization, inverted index construction
- `types.go` — shared types (`doc`, `scorePair`, `scoreResult`) and `getTokenizedText()` tokenizer
- `main_test.go` — benchmarks comparing sequential vs. parallel scoring at various corpus sizes (200–50000 docs)

**Concurrency pattern:** Workers are launched with a `sync.WaitGroup`; each sends its result slice into a buffered channel sized to `numCPU`. A separate goroutine waits on the WaitGroup then closes the channel, allowing the main goroutine to range over it safely (avoids the deadlock that unbuffered channels caused).
