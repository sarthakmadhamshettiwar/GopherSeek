package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

// all the data fetching logic will reside in this file. Whether it is from DB or from a mocked file

func getCorpusFromFile() []doc {
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

func getCorpusFromDB() []doc {
	err := godotenv.Load()

	if err != nil {
		fmt.Printf("Error loading .env file: %v\n", err)
		return []doc{}
	}

	ctx := context.Background()
	connString := os.Getenv("DOCS_DATABASE_URL")
	if connString == "" {
		fmt.Fprintf(os.Stderr, "DOCS_DATABASE_URL not set\n")
		os.Exit(1)
	}

	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		fmt.Printf("Unable to connect to database: %v\n", err)
		return []doc{}
	}

	// Fetch all the documents from the table

	allDocs, err := conn.Query(ctx, "SELECT id, text FROM docs;")
	if err != nil {
		fmt.Printf("Error fetching documents: %v\n", err)
		return []doc{}
	}

	defer allDocs.Close()
	var corpus []doc
	for allDocs.Next() {
		var d doc
		err := allDocs.Scan(&d.id, &d.text)
		if err != nil {
			fmt.Printf("Error scanning row: %v\n", err)
			return []doc{}
		}
		corpus = append(corpus, d)
		fmt.Printf("Fetched doc ID: %s\n", d.text) // Debugging log to confirm data retrieval
	}

	return corpus
}

func getCorpus(source string) []doc {
	switch source {
	case "db":
		return getCorpusFromDB()
	case "file":
		return getCorpusFromFile()
	}

	fmt.Printf("Unknown corpus source: %s\n", source)
	return nil
}

func getTokenizedCorpus(corpus []doc) (map[int][]string, float64, map[string][]int) {
	tokenizedCorpus := make(map[int][]string)
	totalDocsLength := 0
	invertedIndex := make(map[string][]int)

	for _, doc := range corpus {
		tokens := getTokenizedText(doc.text)
		populateInvertedIndex(&invertedIndex, tokens, doc.id)
		tokenizedCorpus[doc.id] = tokens
		totalDocsLength += len(tokens)
	}

	avgDocsLength := float64(totalDocsLength) / float64(len(corpus))
	return tokenizedCorpus, avgDocsLength, invertedIndex
}

func populateInvertedIndex(invertedIndex *map[string][]int, tokens []string, docID int) {
	for _, token := range tokens {
		(*invertedIndex)[token] = append((*invertedIndex)[token], docID)
	}
}
