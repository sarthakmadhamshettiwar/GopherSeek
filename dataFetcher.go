package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
		if v, ok := item["name"].(string); ok {
			d.name = v
		}
		// "description" in JSON → doc.text for BM25
		if v, ok := item["description"].(string); ok {
			d.text = v
		}
		if v, ok := item["lat"].(float64); ok {
			d.lat = v
		}
		if v, ok := item["long"].(float64); ok {
			d.long = v
		}
		corpus = append(corpus, d)
	}

	return corpus
}

func loadDBConfig() (context.Context, context.CancelFunc, string, error) {
	if err := godotenv.Load(); err != nil {
		return nil, nil, "", fmt.Errorf("error loading .env file: %w", err)
	}
	connString := os.Getenv("DOCS_DATABASE_URL")
	if connString == "" {
		fmt.Fprintf(os.Stderr, "DOCS_DATABASE_URL not set\n")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	return ctx, cancel, connString, nil
}

func scanCorpusRows(rows pgx.Rows) []doc {
	defer rows.Close()
	var corpus []doc
	for rows.Next() {
		var d doc
		if err := rows.Scan(&d.id, &d.name, &d.text, &d.lat, &d.long); err != nil {
			fmt.Printf("Error scanning row: %v\n", err)
			return []doc{}
		}
		corpus = append(corpus, d)
	}
	return corpus
}

func getCorpusFromDB() []doc {
	ctx, cancel, connString, err := loadDBConfig()
	if err != nil {
		fmt.Println(err)
		return []doc{}
	}
	defer cancel()

	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		fmt.Printf("Unable to connect to database: %v\n", err)
		return []doc{}
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, "SELECT id, name, text, lat, long FROM docs;")
	if err != nil {
		fmt.Printf("Error fetching documents: %v\n", err)
		return []doc{}
	}
	return scanCorpusRows(rows)
}

func getCorpusFromDBPool() []doc {
	ctx, cancel, connString, err := loadDBConfig()
	if err != nil {
		fmt.Println(err)
		return []doc{}
	}
	defer cancel()

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		fmt.Printf("Unable to create connection pool: %v\n", err)
		return []doc{}
	}
	defer pool.Close()

	rows, err := pool.Query(ctx, "SELECT id, name, text, lat, long FROM docs;")
	if err != nil {
		fmt.Printf("Error fetching documents: %v\n", err)
		return []doc{}
	}
	return scanCorpusRows(rows)
}

func getCorpus(source string) []doc {
	switch source {
	case "db":
		if os.Getenv("USE_POOL") == "true" {
			return getCorpusFromDBPool()
		}
		return getCorpusFromDB()
	case "file":
		return getCorpusFromFile()
	}

	fmt.Printf("Unknown corpus source: %s\n", source)
	return nil
}

// geohashPrecision controls the pre-filter granularity.
// Precision 4 ≈ 40×20km cells; 9-cell neighborhood covers ~120×60km, encompassing a city region.
const geohashPrecision = 4


func getTokenizedCorpus(corpus []doc) (map[int][]string, float64, map[string][]int, map[int]docMeta, map[string][]int)  {
	tokenizedCorpus := make(map[int][]string) // docID -> tokenized text
	invertedIndex := make(map[string][]int) // token -> list of docIDs
	locationIndex := make(map[int]docMeta) // docID -> document metadata
	geohashIndex := make(map[string][]int) // geohash -> list of docIDs
	totalDocsLength := 0

	for _, d := range corpus {
		tokens := getTokenizedText(d.text)
		populateInvertedIndex(&invertedIndex, tokens, d.id)
		tokenizedCorpus[d.id] = tokens
		totalDocsLength += len(tokens)

		locationIndex[d.id] = docMeta{
			lat:  d.lat,
			long: d.long,
			name: d.name,
			text: d.text,
		}

		hash := encodeGeohash(d.lat, d.long, geohashPrecision)
		geohashIndex[hash] = append(geohashIndex[hash], d.id)
	}

	if len(corpus) == 0 {
		return tokenizedCorpus, 0, invertedIndex, locationIndex, geohashIndex
	}
	avgDocsLength := float64(totalDocsLength) / float64(len(corpus))
	return tokenizedCorpus, avgDocsLength, invertedIndex, locationIndex, geohashIndex
}

// add docId for all the tokens in the current document to the inverted index
func populateInvertedIndex(invertedIndex *map[string][]int, tokens []string, docID int) {
	for _, token := range tokens {
		(*invertedIndex)[token] = append((*invertedIndex)[token], docID)
	}
}
