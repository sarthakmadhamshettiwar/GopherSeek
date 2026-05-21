package application

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"runtime"
	"sort"
	"strconv"
	"sync"

	"search/data"
)

func getTokenizedCorpus(corpus []data.Doc) (map[int][]string, float64, map[string][]int, map[int]docMeta, map[string][]int) {
	tokenizedCorpus := make(map[int][]string)
	invertedIndex := make(map[string][]int)
	locationIndex := make(map[int]docMeta)
	geohashIndex := make(map[string][]int)
	totalDocsLength := 0

	for _, d := range corpus {
		tokens := getTokenizedText(d.Text)
		populateInvertedIndex(&invertedIndex, tokens, d.Id)
		tokenizedCorpus[d.Id] = tokens
		totalDocsLength += len(tokens)

		locationIndex[d.Id] = docMeta{
			lat:  d.Lat,
			long: d.Long,
			name: d.Name,
			text: d.Text,
		}

		hash := encodeGeohash(d.Lat, d.Long, geohashPrecision)
		geohashIndex[hash] = append(geohashIndex[hash], d.Id)
	}

	if len(corpus) == 0 {
		return tokenizedCorpus, 0, invertedIndex, locationIndex, geohashIndex
	}
	avgDocsLength := float64(totalDocsLength) / float64(len(corpus))
	return tokenizedCorpus, avgDocsLength, invertedIndex, locationIndex, geohashIndex
}

func populateInvertedIndex(invertedIndex *map[string][]int, tokens []string, docID int) {
	for _, token := range tokens {
		(*invertedIndex)[token] = append((*invertedIndex)[token], docID)
	}
}

func GetDocumentScoresByIdParallel(query string, candidateIDs []int, tokenizedCorpus map[int][]string, invertedIndex map[string][]int, avgDocsLength float64) map[int]float64 {
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

func GetDocumentScoresByIdSequential(query string, candidateIDs []int, tokenizedCorpus map[int][]string, invertedIndex map[string][]int, avgDocsLength float64) map[int]float64 {
	scores := make(map[int]float64)
	totalDocs := len(tokenizedCorpus)
	for _, id := range candidateIDs {
		scores[id] = computeRelevanceScore(query, tokenizedCorpus[id], invertedIndex, totalDocs, avgDocsLength)
	}
	return scores
}

func getTopSearchResults(gq GeoQuery, tokenizedCorpus map[int][]string, invertedIndex map[string][]int, avgDocsLength float64, locationIndex map[int]docMeta, geohashIndex map[string][]int) []SearchResult {
	userHash := encodeGeohash(gq.UserLat, gq.UserLong, geohashPrecision)
	neighborHashes := geohashNeighbors(userHash)
	allCells := append(neighborHashes, userHash)

	var candidateIDs []int
	for _, h := range allCells {
		candidateIDs = append(candidateIDs, geohashIndex[h]...)
	}

	scoresByID := GetDocumentScoresByIdParallel(gq.Text, candidateIDs, tokenizedCorpus, invertedIndex, avgDocsLength)

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

	results := make([]SearchResult, 0, len(bm25Results))
	for _, r := range bm25Results {
		meta := locationIndex[r.id]
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

func parseGeoQuery(params url.Values) (GeoQuery, error) {
	queryText := params.Get("query")
	if queryText == "" {
		return GeoQuery{}, fmt.Errorf("query is required")
	}

	latStr, longStr := params.Get("lat"), params.Get("long")
	if latStr == "" || longStr == "" {
		return GeoQuery{}, fmt.Errorf("lat and long are required")
	}

	userLat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		return GeoQuery{}, fmt.Errorf("invalid lat")
	}
	userLong, err := strconv.ParseFloat(longStr, 64)
	if err != nil {
		return GeoQuery{}, fmt.Errorf("invalid long")
	}

	radiusKm := 0.0
	if rs := params.Get("radius"); rs != "" {
		if radiusKm, err = strconv.ParseFloat(rs, 64); err != nil {
			return GeoQuery{}, fmt.Errorf("invalid radius")
		}
	}

	topK := 10
	if ks := params.Get("k"); ks != "" {
		if parsed, err := strconv.Atoi(ks); err == nil && parsed > 0 {
			topK = parsed
		}
	}

	sortBy := "relevancy"
	if params.Get("sort") == "distance" {
		sortBy = "distance"
	}

	return GeoQuery{Text: queryText, UserLat: userLat, UserLong: userLong, RadiusKm: radiusKm, TopK: topK, SortBy: sortBy}, nil
}

func searchHandler(tokenizedCorpus map[int][]string, avgDocsLength float64, invertedIndex map[string][]int, locationIndex map[int]docMeta, geohashIndex map[string][]int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gq, err := parseGeoQuery(r.URL.Query())
		if err != nil {
			slog.Warn("bad search request", "error", err, "remote", r.RemoteAddr)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		slog.Info("search request", "query", gq.Text, "lat", gq.UserLat, "long", gq.UserLong, "radius_km", gq.RadiusKm, "top_k", gq.TopK, "sort", gq.SortBy)
		results := getTopSearchResults(gq, tokenizedCorpus, invertedIndex, avgDocsLength, locationIndex, geohashIndex)
		slog.Info("search results", "query", gq.Text, "count", len(results))

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if err := json.NewEncoder(w).Encode(results); err != nil {
			slog.Error("failed to encode response", "error", err)
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

func Run() {
	corpus := data.GetCorpus("db")
	slog.Info("indexing corpus", "docs", len(corpus))
	tokenizedCorpus, avgDocsLength, invertedIndex, locationIndex, geohashIndex := getTokenizedCorpus(corpus)
	slog.Info("index built", "terms", len(invertedIndex), "avg_doc_len", avgDocsLength)

	http.HandleFunc("/search", searchHandler(tokenizedCorpus, avgDocsLength, invertedIndex, locationIndex, geohashIndex))
	http.Handle("/", http.FileServer(http.Dir("./frontend/")))

	// For dev
	http.HandleFunc("/inverted-index", invertedIndexHandler(invertedIndex))

	slog.Info("server starting", "addr", ":8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		slog.Error("server failed", "error", err)
	}
}
