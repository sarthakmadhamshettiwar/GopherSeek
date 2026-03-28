package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync"
)

func getDocumentScoresByIdParallel(query string, tokenizedCorpus map[int][]string, invertedIndex map[string][]int, avgDocsLength float64) map[int]float64 {
	scores := make(map[int]float64)
	totalDocs := len(tokenizedCorpus)
	numWorkers := runtime.NumCPU()
	chunkSize := (totalDocs + numWorkers - 1) / numWorkers

	resultsChan := make(chan []scoreResult)
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerIdx int) {
			defer wg.Done()
			start := workerIdx * chunkSize
			end := start + chunkSize
			if end > totalDocs {
				end = totalDocs
			}

			// Pre-allocate the slice for this chunk to avoid resizing
			localScores := make([]scoreResult, 0, chunkSize)
			for id := start; id < end; id++ {
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

func getDocumentScoresByIdSequential(query string, tokenizedCorpus map[int][]string, invertedIndex map[string][]int, avgDocsLength float64) map[int]float64 {
	scores := make(map[int]float64)
	totalDocs := len(tokenizedCorpus)
	for id := range tokenizedCorpus {
		scores[id] = computeRelevanceScore(query, tokenizedCorpus[id], invertedIndex, totalDocs, avgDocsLength)
	}
	return scores
}

func getTopSearchResults(query string, tokenizedCorpus map[int][]string, invertedIndex map[string][]int, avgDocsLength float64, topN int, thresholdScore float64) []scorePair {

	scoresByIds := getDocumentScoresByIdParallel(query, tokenizedCorpus, invertedIndex, avgDocsLength)

	// Sort the document IDs by their scores
	var scoredDocs []scorePair
	for id, score := range scoresByIds {
		scoredDocs = append(scoredDocs, scorePair{Id: id, Score: score, Text: strings.Join(tokenizedCorpus[id], " ")})
	}

	sort.Slice(scoredDocs, func(i, j int) bool {
		return scoredDocs[i].Score > scoredDocs[j].Score
	})

	// Get the top N document IDs
	var topDocs []scorePair
	for i := 0; i < topN && i < len(scoredDocs) && scoredDocs[i].Score > thresholdScore; i++ {
		topDocs = append(topDocs, scorePair{Id: scoredDocs[i].Id, Score: scoredDocs[i].Score, Text: scoredDocs[i].Text})
	}
	return topDocs
}

func searchHandler(tokenizedCorpus map[int][]string, avgDocsLength float64, invertedIndex map[string][]int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		params := r.URL.Query()
		query := params["query"]

		fmt.Printf("Received search query: %v\n", query)

		topSearchResults := getTopSearchResults(query[0], tokenizedCorpus, invertedIndex, avgDocsLength, 10, 0)
		json.NewEncoder(w).Encode(topSearchResults)
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
	tokenizedCorpus, avgDocsLength, invertedIndex := getTokenizedCorpus(getCorpus("db"))

	// search endpoint
	http.HandleFunc("/search", searchHandler(tokenizedCorpus, avgDocsLength, invertedIndex))

	// get the inverted index (for debugging)
	http.HandleFunc("/inverted-index", invertedIndexHandler(invertedIndex))
	fmt.Println("Server is starting!")
	err := http.ListenAndServe(":8080", nil)

	if err != nil {
		fmt.Printf("Error starting server: %v\n", err)
	}
}
