# Geo-Search Implementation Plan

## Overview

Extend GopherSeek to support geographical search by combining BM25 text relevance with geographic distance. The approach:

1. **User location is required** — requests without `lat` and `long` are rejected with HTTP 400
2. **Geohash** pre-filters candidates to a ~50km radius around the user before BM25 runs
3. Fetch **top K results purely by BM25** (text relevance first, on the geo-filtered candidates)
4. Calculate **Haversine distance** for each of the top K results
5. Let the user **sort by distance or relevancy** — no score blending
6. Optionally let the user **filter by radius** (opt-in, applied after BM25)

---

## Search Flow

```
User query + user lat/long (required)
        ↓
lat/long missing? → return HTTP 400 "Location required"
        ↓
Geohash → narrow candidate docs within ~50km radius
        ↓
BM25 on description → top K results from candidates
        ↓
Calculate Haversine distance for each result
        ↓
Optional: User sets radius → hard filter applied
        ↓
User sorts: distance OR relevancy
```

---

## Step 1: Redesign `corpus.json`

Replace the existing corpus with Delhi-area places that include coordinates. Design the data so that multiple places have similar descriptions but different locations — this makes geo-ranking clearly observable when testing.

```json
[
  {
    "id": 1,
    "name": "Lodhi Garden",
    "description": "Peaceful and less crowded park great for morning walks",
    "lat": 28.5931,
    "long": 77.2197
  },
  {
    "id": 2,
    "name": "Sanjay Van",
    "description": "Peaceful forested area less crowded and quiet",
    "lat": 28.5530,
    "long": 77.1833
  }
]
```

The `name` field is for display only — BM25 runs on `description`.

---

## Step 2: Update the `doc` Type

**File:** `types.go`

```go
type doc struct {
    id          int
    name        string
    text        string
    lat         float64
    long        float64
}
```

Also add a new struct for the search query:

```go
type GeoQuery struct {
    Text     string
    UserLat  float64
    UserLong float64
    RadiusKm float64  // 0 means no radius filter
    TopK     int      // number of BM25 results to fetch, default 10
    SortBy   string   // "relevancy" or "distance"
}
```

---

## Step 3: Update `getCorpusFromFile()` to Read `lat`, `long`, and `name`

**File:** `dataFetcher.go`

Update the JSON unmarshalling to include `name`, `lat`, and `long` fields from the redesigned `corpus.json`.

---

## Step 4: Add Geohash + Haversine in `geo.go`

**File:** New file `geo.go`

### Haversine Distance

```go
// haversineDistance returns distance in kilometers between two lat/long points.
func haversineDistance(lat1, long1, lat2, long2 float64) float64
```

Pure `math` package — no external library needed.

### Geohash

```go
// encodeGeohash encodes a lat/long into a geohash string of given precision.
func encodeGeohash(lat, long float64, precision int) string

// geohashNeighbors returns the 8 neighboring geohash cells + the cell itself.
func geohashNeighbors(hash string) []string
```

Use geohash for **candidate filtering** — only score docs whose geohash cell is within ~50km of the user. Docs outside this range are excluded before BM25 runs. This is a hard boundary, not a soft penalty.

> **Note on the boundary problem:** Two points physically adjacent but on opposite sides of a geohash cell boundary will have different hashes. Always check the 8 neighboring cells alongside the center cell to handle this correctly.

---

## Step 5: Store Location and Geohash Index In-Memory

**File:** `dataFetcher.go`

Add two new maps built at startup:

```go
locationIndex map[int][2]float64  // docID → {lat, long}
geohashIndex  map[string][]int    // geohash → []docID
```

Update `getTokenizedCorpus()` signature:

```go
func getTokenizedCorpus(corpus []doc) (
    map[int][]string,
    float64,
    map[string][]int,
    map[int][2]float64,
    map[string][]int,  // geohashIndex
)
```

Use precision 5 for geohash (~5km cells) — good balance for the radius use case.

---

## Step 6: Update Scoring in `search.go`

Update `getTopSearchResults()` to accept a `GeoQuery` and both new indexes.

### Logic:

1. Use `geohashNeighbors` on the user's location to get candidate doc IDs within ~50km. Only run BM25 on those candidates.
2. Take top K by BM25 score.
3. For each of the top K results, compute `haversineDistance` and attach it to the result.
4. If `RadiusKm > 0`, hard-exclude docs beyond that radius.
5. Sort the final list by `SortBy`:
   - `"relevancy"` → sort by BM25 score descending
   - `"distance"` → sort by distance ascending

---

## Step 7: Update the `/search` HTTP Handler

**File:** `search.go` — `searchHandler()`

Accept the following query params:

| Param    | Type    | Description                        | Default      |
|----------|---------|------------------------------------|--------------|
| `query`  | string  | Search text                        | required     |
| `lat`    | float64 | User's current latitude            | required     |
| `long`   | float64 | User's current longitude           | required     |
| `radius` | float64 | Max distance in km (optional)      | 0 (no limit) |
| `k`      | int     | Number of top BM25 results         | 10           |
| `sort`   | string  | `"relevancy"` or `"distance"`      | `"relevancy"`|

If `lat` or `long` are missing or unparseable, return HTTP 400 with message: `"lat and long are required"`.

Parse `radius` using `strconv.ParseFloat`. If parsing fails, return HTTP 400.

---

## Step 8: Update `main()` to Pass New Indexes

**File:** `search.go`

```go
tokenizedCorpus, avgDocsLength, invertedIndex, locationIndex, geohashIndex := getTokenizedCorpus(getCorpus("file"))
```

Pass `locationIndex` and `geohashIndex` into `searchHandler()`.

---

## Step 9: Update HTTP Response Format

Return JSON so the frontend (website) can handle sorting and display cleanly:

```json
[
  {
    "id": 7,
    "name": "Lodhi Garden",
    "text": "Peaceful and less crowded park",
    "bm25Score": 0.82,
    "distanceKm": 1.3,
    "lat": 28.59,
    "long": 77.21
  }
]
```

Both `bm25Score` and `distanceKm` are always returned — the client can re-sort on either without a new request.

---

## Step 10: Frontend (Website)

A simple browser-based UI that:

1. Asks for location permission via `navigator.geolocation.getCurrentPosition()` **before activating the search bar** — location is required to use the app
2. If permission is denied, show a clear message: "Location access is required to search"
3. Sends `lat` and `long` as query params to the Go backend
4. Displays results with toggle buttons — **Sort by Distance** / **Sort by Relevancy**
5. Optionally exposes a radius input field

For local testing on mobile: use **ngrok** to tunnel the local Go server over HTTPS (required for browser Geolocation API to work on mobile).

---

## File Change Summary

| File            | Change                                                                        |
|-----------------|-------------------------------------------------------------------------------|
| `corpus.json`   | Redesign with Delhi places, coordinates, and name field                       |
| `types.go`      | Add `name`, `lat`, `long` to `doc`; replace old query struct with `GeoQuery` |
| `dataFetcher.go`| Update JSON parsing; return `locationIndex` and `geohashIndex`                |
| `geo.go`        | New file: `haversineDistance`, `encodeGeohash`, `geohashNeighbors`            |
| `bm25.go`       | No change                                                                     |
| `search.go`     | Update handler params, scoring logic, sorting, `main()`                       |
| `frontend/`     | New: simple HTML/JS website with geolocation + sort toggle                    |

---

## What is NOT in scope

- No blended scoring (`k * bm25 + (1-k) * distance`) — user chooses sort explicitly
- No SSEs or live location tracking
- No persistent storage of user locations
- No changes to the BM25 algorithm itself
- No ORM or migration tooling