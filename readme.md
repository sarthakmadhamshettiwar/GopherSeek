# GopherSeek

A full-text search engine written in Go that combines **BM25 relevance scoring** with **geohash-based geo-filtering**. Given a query and a user location, it returns the most relevant documents within a configurable radius, ranked by either relevance or distance.

## Features

- BM25 ranking (k=1.2, b=0.75) over a tokenized corpus
- Geohash pre-filter + Haversine distance for geo-aware search
- Parallel scoring across `runtime.NumCPU()` workers
- Pluggable corpus source: local JSON file or PostgreSQL
- Minimal React frontend served from the same Go binary

## Installation

### Prerequisites

- **Go** 1.26 or newer
- **Node.js** 18+ (only if you want to rebuild the frontend)
- **PostgreSQL** (optional — only if you want to load the corpus from a database instead of the bundled `corpus.json`)

### Steps

1. Clone the repository:
   ```bash
   git clone <repo-url>
   cd GopherSeek
   ```

2. Install Go dependencies:
   ```bash
   go mod download
   ```

3. (Optional) If you want to use PostgreSQL as the corpus source, create a `.env` file in the project root:
   ```
   DOCS_DATABASE_URL=postgres://user:password@host:5432/dbname
   ```
   The expected table schema is:
   ```sql
   CREATE TABLE docs (
       id    INT PRIMARY KEY,
       name  TEXT,
       text  TEXT,
       lat   DOUBLE PRECISION,
       long  DOUBLE PRECISION
   );
   ```

4. (Optional) If you want to modify the frontend:
   ```bash
   cd frontend
   npm install
   ```

## Running the project

From the project root:

```bash
go run .
```

The server starts on **http://localhost:8080**.

By default it loads documents from `corpus.json`. To switch to PostgreSQL, change the call inside `main()` in `search.go`:

```go
getCorpus("db")   // instead of getCorpus("file")
```

To build a standalone binary:

```bash
go build
./search
```

## Usage

### Web UI

Open **http://localhost:8080** in your browser. The bundled frontend lets you enter a query, set your location, choose a radius, and view ranked results on a map/list.

### HTTP API

#### `GET /search`

Search the corpus near a given location.

**Query parameters:**

| Param    | Required | Description                                            |
|----------|----------|--------------------------------------------------------|
| `query`  | yes      | Search text (e.g. `pizza`)                             |
| `lat`    | yes      | User latitude (float)                                  |
| `long`   | yes      | User longitude (float)                                 |
| `radius` | no       | Hard distance filter in km. `0` or omitted = no filter |
| `k`      | no       | Number of top results to return (default `10`)         |
| `sort`   | no       | `relevancy` (default) or `distance`                    |

**Example:**

```bash
curl "http://localhost:8080/search?query=pizza&lat=28.6139&long=77.2090&radius=5&k=5&sort=relevancy"
```

**Response:**

```json
[
  {
    "id": 42,
    "name": "Tony's Pizzeria",
    "text": "Wood-fired pizza in central Delhi...",
    "bm25Score": 7.21,
    "distanceKm": 1.34,
    "lat": 28.6201,
    "long": 77.2151
  }
]
```

#### `GET /inverted-index`

Returns the in-memory token → document-ID inverted index in plain text. Useful for debugging which terms map to which documents.

```bash
curl http://localhost:8080/inverted-index
```

## Testing & Benchmarks

```bash
# Run all tests
go test

# Run benchmarks (sequential vs parallel scoring at various corpus sizes)
go test -bench=. -benchmem

# Run a specific benchmark
go test -bench=BenchmarkParallel -benchmem
```

### Database connection benchmarks

These benchmarks require `DOCS_DATABASE_URL` to be set in a `.env` file (see Installation). They compare three DB connection patterns under 100 concurrent queries.

```bash
# Run all three connection pattern benchmarks (cold burst)
go test -bench=BenchmarkDB_DirectConnection_100Concurrent\|BenchmarkDB_SharedPool_100Concurrent\|BenchmarkDB_PoolPerCall_100Concurrent -benchmem -count=3 -timeout=10m

# Run multi-wave benchmarks (5 waves of 100 concurrent queries)
# This is where the pool's reuse advantage becomes visible
go test -bench=BenchmarkDB_DirectConnection_MultiWave\|BenchmarkDB_SharedPool_MultiWave -benchmem -count=3 -timeout=15m

# Run the pre-warmed pool benchmark (steady-state, simulates a running server)
go test -bench=BenchmarkDB_SharedPool_Warmed -benchmem -count=3 -timeout=10m

# Run all DB benchmarks at once
go test -bench=BenchmarkDB -benchmem -count=3 -timeout=15m
```

**What each benchmark measures:**

| Benchmark | What it simulates |
|---|---|
| `DirectConnection_100Concurrent` | 100 goroutines, each opens its own connection per query |
| `SharedPool_100Concurrent` | One shared pool (MaxConns=100), 100 concurrent queries |
| `PoolPerCall_100Concurrent` | Anti-pattern: a new pool created per call (for comparison) |
| `DirectConnection_MultiWave` | 5 sequential bursts of 100 queries — no connection reuse |
| `SharedPool_MultiWave` | 5 sequential bursts — connections from burst 1 reused in 2–5 |
| `SharedPool_Warmed` | Pool pre-warmed before timing — pure steady-state query cost |
