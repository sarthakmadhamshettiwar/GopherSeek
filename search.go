package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"sync"
)

func getDocumentScoresByIdParallel(query string, candidateIDs []int, tokenizedCorpus map[int][]string, invertedIndex map[string][]int, avgDocsLength float64) map[int]float64 {
	scores := make(map[int]float64)
	n := len(candidateIDs)
	if n == 0 {
		return scores
	}
	totalDocs := len(tokenizedCorpus)
	numWorkers := runtime.NumCPU()
	chunkSize := (n + numWorkers - 1) / numWorkers

	resultsChan := make(chan []scoreResult, numWorkers)
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerIdx int) {
			defer wg.Done()
			start := workerIdx * chunkSize
			if start >= n {
				resultsChan <- nil
				return
			}
			end := start + chunkSize
			if end > n {
				end = n
			}
			localScores := make([]scoreResult, 0, end-start)
			for _, id := range candidateIDs[start:end] {
				score := computeRelevanceScore(query, tokenizedCorpus[id], invertedIndex, totalDocs, avgDocsLength)
				localScores = append(localScores, scoreResult{id: id, score: score})
			}

			resultsChan <- localScores
		}(i)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	for res := range resultsChan {
		for _, result := range res {
			scores[result.id] = result.score
		}
	}
	return scores
}

func getDocumentScoresByIdSequential(query string, candidateIDs []int, tokenizedCorpus map[int][]string, invertedIndex map[string][]int, avgDocsLength float64) map[int]float64 {
	scores := make(map[int]float64)
	totalDocs := len(tokenizedCorpus)
	for _, id := range candidateIDs {
		scores[id] = computeRelevanceScore(query, tokenizedCorpus[id], invertedIndex, totalDocs, avgDocsLength)
	}
	return scores
}

func getTopSearchResults(gq GeoQuery, tokenizedCorpus map[int][]string, invertedIndex map[string][]int, avgDocsLength float64, locationIndex map[int]docMeta, geohashIndex map[string][]int) []SearchResult {
	// 1. Geohash pre-filter: collect candidate doc IDs within the neighborhood cells.
	userHash := encodeGeohash(gq.UserLat, gq.UserLong, geohashPrecision)
	neighborHashes := geohashNeighbors(userHash)

	// combine userHash + neighbourhood hashes
	allCells := append(neighborHashes, userHash)

	candidateSet := make(map[int]struct{})
	for _, h := range allCells {
		for _, id := range geohashIndex[h] {
			candidateSet[id] = struct{}{}
		}
	}

	candidateIDs := make([]int, 0, len(candidateSet))
	for id := range candidateSet {
		candidateIDs = append(candidateIDs, id)
	}

	// 2. BM25 on geo-filtered candidates only.
	scoresByID := getDocumentScoresByIdParallel(gq.Text, candidateIDs, tokenizedCorpus, invertedIndex, avgDocsLength)

	// 3. Sort by BM25 descending, take top K.
	type idScore struct {
		id    int
		score float64
	}
	bm25Results := make([]idScore, 0, len(scoresByID))
	for id, score := range scoresByID {
		if score > 0 {
			bm25Results = append(bm25Results, idScore{id, score})
		}
	}
	sort.Slice(bm25Results, func(i, j int) bool {
		if bm25Results[i].score != bm25Results[j].score {
			return bm25Results[i].score > bm25Results[j].score
		}
		return bm25Results[i].id < bm25Results[j].id
	})
	if len(bm25Results) > gq.TopK {
		bm25Results = bm25Results[:gq.TopK]
	}

	// 4. Compute Haversine distance; apply optional radius hard-filter.
	results := make([]SearchResult, 0, len(bm25Results))
	for _, r := range bm25Results {
		meta := locationIndex[r.id] // contains lat/long + other doc metadata for docId
		distKm := haversineDistance(gq.UserLat, gq.UserLong, meta.lat, meta.long)
		if gq.RadiusKm > 0 && distKm > gq.RadiusKm {
			continue
		}
		results = append(results, SearchResult{
			ID:         r.id,
			Name:       meta.name,
			Text:       meta.text,
			BM25Score:  r.score,
			DistanceKm: distKm,
			Lat:        meta.lat,
			Long:       meta.long,
		})
	}

	// 5. Sort by requested field (already sorted by BM25; re-sort only if distance requested).
	if gq.SortBy == "distance" {
		sort.Slice(results, func(i, j int) bool {
			if results[i].DistanceKm != results[j].DistanceKm {
				return results[i].DistanceKm < results[j].DistanceKm
			}
			return results[i].ID < results[j].ID
		})
	}

	return results
}

func searchHandler(tokenizedCorpus map[int][]string, avgDocsLength float64, invertedIndex map[string][]int, locationIndex map[int]docMeta, geohashIndex map[string][]int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := r.URL.Query()

		queryText := params.Get("query")
		if queryText == "" {
			http.Error(w, "query is required", http.StatusBadRequest)
			return
		}

		latStr := params.Get("lat")
		longStr := params.Get("long")
		if latStr == "" || longStr == "" {
			http.Error(w, "lat and long are required", http.StatusBadRequest)
			return
		}

		userLat, err := strconv.ParseFloat(latStr, 64)
		if err != nil {
			http.Error(w, "invalid lat", http.StatusBadRequest)
			return
		}
		userLong, err := strconv.ParseFloat(longStr, 64)
		if err != nil {
			http.Error(w, "invalid long", http.StatusBadRequest)
			return
		}

		radiusKm := 0.0
		if rs := params.Get("radius"); rs != "" {
			radiusKm, err = strconv.ParseFloat(rs, 64)
			if err != nil {
				http.Error(w, "invalid radius", http.StatusBadRequest)
				return
			}
		}

		topK := 10
		if ks := params.Get("k"); ks != "" {
			if parsed, err := strconv.Atoi(ks); err == nil && parsed > 0 {
				topK = parsed
			}
		}

		sortBy := params.Get("sort")
		if sortBy != "distance" {
			sortBy = "relevancy"
		}

		gq := GeoQuery{
			Text:     queryText,
			UserLat:  userLat,
			UserLong: userLong,
			RadiusKm: radiusKm,
			TopK:     topK,
			SortBy:   sortBy,
		}

		results := getTopSearchResults(gq, tokenizedCorpus, invertedIndex, avgDocsLength, locationIndex, geohashIndex)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if err := json.NewEncoder(w).Encode(results); err != nil {
			fmt.Printf("Error encoding response: %v\n", err)
		}
	}
}

func invertedIndexHandler(invertedIndex map[string][]int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Inverted Index:\n")
		for term, docIDs := range invertedIndex {
			fmt.Fprintf(w, "  %s: %v\n", term, docIDs)
		}
	}
}

func main() {
	tokenizedCorpus, avgDocsLength, invertedIndex, locationIndex, geohashIndex := getTokenizedCorpus(getCorpus("file"))

	http.HandleFunc("/search", searchHandler(tokenizedCorpus, avgDocsLength, invertedIndex, locationIndex, geohashIndex))
	http.Handle("/", http.FileServer(http.Dir("./frontend/")))

	// For dev
	http.HandleFunc("/inverted-index", invertedIndexHandler(invertedIndex))

	fmt.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
	}
}
