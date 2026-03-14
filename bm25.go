package main

import (
	"math"
)

func getIDFForToken(queryToken string, invertedIndex map[string][]int, totalDocs int) float64 {

	docsWithToken := len(invertedIndex[queryToken])
	if docsWithToken == 0 {
		return 0
	}
	return math.Log((float64(totalDocs) - float64(docsWithToken) + 0.5) / (float64(docsWithToken) + 0.5))
}

func getIDFForQuery(query string, invertedIndex map[string][]int, totalDocs int) float64 {
	tokenizedQuery := getTokenizedText(query) // ['nike', 'shoes']
	idf := 0.0
	for _, token := range tokenizedQuery {
		idf += getIDFForToken(token, invertedIndex, totalDocs)
	}
	return idf
}

func getTFForToken(token string, docTokens []string) int {
	termFrequency := 0
	for _, tokenInDoc := range docTokens {
		if tokenInDoc == token {
			termFrequency++
		}
	}
	return termFrequency
}

func getTFForQuery(query string, docTokens []string, avgDocsLength float64) float64 {
	currentDocLen := len(docTokens)
	k := 1.2  // saturate term frequency to prevent bias towards longer documents as the TF might grow linearly with the document length
	b := 0.75 // controls the impact of document length normalization: if same token appears in two documents with same term frequency, the shorter document will be scored higher.
	// If b is 0, there is no length normalization and if b is 1, there is full length normalization.

	// TODO: will need a better way to implement it. This implementation is not suitable for multi-tokens query
	queryTokens := getTokenizedText(query) // [nike, shoes]
	tf := 0.0
	for _, token := range queryTokens {
		tf += float64(getTFForToken(token, docTokens))
	}
	return (tf * (k + 1)) / (tf + (k * (1 - b + b*float64(currentDocLen)/avgDocsLength)))
}

// Calculate the relevancy of a document for a given query
func computeRelevanceScore(query string, docTokens []string, invertedIndex map[string][]int, totalDocs int, avgDocsLength float64) float64 {
	idf := getIDFForQuery(query, invertedIndex, totalDocs)
	tf := getTFForQuery(query, docTokens, avgDocsLength)
	return tf * idf
}
