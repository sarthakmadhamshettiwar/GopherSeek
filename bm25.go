package main

import (
	"math"
)

func getIDFForToken(token string, tokenizedCorpus map[int][]string) float64 {
	totalDocs := len(tokenizedCorpus)
	docsWithToken := 0
	for _, tokens := range tokenizedCorpus {
		for _, t := range tokens {
			if t == token {
				docsWithToken++
				break
			}
		}
	}
	if docsWithToken == 0 {
		return 0
	}

	return math.Log((float64(totalDocs) - float64(docsWithToken) + 0.5) / (float64(docsWithToken) + 0.5))
}

func getIDFForQuery(query string, tokenizedCorpus map[int][]string) float64 {
	tokens := getTokenizedText(query)
	idf := 0.0
	for _, token := range tokens {
		idf += getIDFForToken(token, tokenizedCorpus)
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

func getTFForQuery(query string, docTokens []string) float64 {
	// TODO: will need a better way to implement it. This implementation is not suitable for multi-tokens query
	queryTokens := getTokenizedText(query) // [nike, shoes]
	tf := 0.0
	for _, token := range queryTokens {
		tf += float64(getTFForToken(token, docTokens))
	}
	return tf
}

// Finds how relevant is query for a given document by calculating the TF-IDF score
func computeRelevanceScore(query string, docTokens []string, tokenizedCorpus map[int][]string) float64 {
	idf := getIDFForQuery(query, tokenizedCorpus)
	tf := getTFForQuery(query, docTokens)
	return tf * idf
}
