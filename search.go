package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

func getDocumentScoresById(query string, tokenizedCorpus map[int][]string, avgDocsLength float64) map[int]float64 {
	scores := make(map[int]float64)
	for id := range tokenizedCorpus {
		scores[id] = computeRelevanceScore(query, tokenizedCorpus[id], tokenizedCorpus, avgDocsLength)
	}
	return scores
}

func getTopSearchResults(query string, topN int, thresholdScore float64) []scorePair {
	tokenizedCorpus, avgDocsLength := getTokenizedCorpus(getCorpus())
	scoresByIds := getDocumentScoresById(query, tokenizedCorpus, avgDocsLength)

	// Sort the document IDs by their scores

	var scoredDocs []scorePair
	for id, score := range scoresByIds {
		scoredDocs = append(scoredDocs, scorePair{id: id, score: score, text: strings.Join(tokenizedCorpus[id], " ")})
	}

	sort.Slice(scoredDocs, func(i, j int) bool {
		return scoredDocs[i].score > scoredDocs[j].score
	})

	// Get the top N document IDs
	var topDocs []scorePair
	for i := 0; i < topN && i < len(scoredDocs) && scoredDocs[i].score > thresholdScore; i++ {
		topDocs = append(topDocs, scorePair{id: scoredDocs[i].id, score: scoredDocs[i].score, text: scoredDocs[i].text})
	}
	return topDocs
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	query := params["query"]

	fmt.Printf("Received search query: %v\n", query)

	topSearchResults := getTopSearchResults(query[0], 10, 0)
	fmt.Fprintf(w, "Search results for: %v\n", query)
	for _, res := range topSearchResults {
		fmt.Fprintf(w, "%v\n", res)
	}
}
func main() {
	http.HandleFunc("/search", searchHandler)
	fmt.Println("Server is starting!")
	err := http.ListenAndServe(":8080", nil)

	if err != nil {
		fmt.Printf("Error starting server: %v\n", err)
	}
}
