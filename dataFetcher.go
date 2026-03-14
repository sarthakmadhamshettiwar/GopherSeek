package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// all the data fetching logic will reside in this file. Whether it is from DB or from a mocked file
func getCorpus() []doc {
	file, err := os.ReadFile("corpus.json")
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return nil
	}

	// Unmarshal into an intermediate representation so we can populate the unexported fields
	var raw []map[string]interface{}
	if err := json.Unmarshal(file, &raw); err != nil {
		fmt.Printf("Error parsing JSON: %v\n", err)
		return nil
	}

	var corpus []doc
	for _, item := range raw {
		var d doc
		// id in JSON will be decoded as float64 for numbers
		if idv, ok := item["id"]; ok {
			switch v := idv.(type) {
			case float64:
				d.id = int(v)
			case int:
				d.id = v
			}
		}
		if tv, ok := item["text"].(string); ok {
			d.text = tv
		}
		corpus = append(corpus, d)
	}

	return corpus
}


func getTokenizedCorpus(corpus []doc) (map[int][]string, float64) {
	tokenizedCorpus := make(map[int][]string)
	totalDocsLength := 0

	for _, d := range corpus {
		tokens := getTokenizedText(d.text)
		tokenizedCorpus[d.id] = tokens
		totalDocsLength += len(tokens)
	}

	avgDocsLength := float64(totalDocsLength) / float64(len(corpus))
	return tokenizedCorpus, avgDocsLength
}
