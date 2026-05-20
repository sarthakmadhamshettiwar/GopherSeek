package main

import (
	"strings"
)

type doc struct {
	id   int
	name string
	text string
	lat  float64
	long float64
}

type docMeta struct {
	lat  float64
	long float64
	name string
	text string
}

type scoreResult struct {
	id    int
	score float64
}

type GeoQuery struct {
	Text     string
	UserLat  float64
	UserLong float64
	RadiusKm float64 // 0 means no radius filter
	TopK     int     // number of top BM25 results to evaluate, default 10
	SortBy   string  // "relevancy" or "distance"
}

type SearchResult struct {
	ID         int     `json:"id"`
	Name       string  `json:"name"`
	Text       string  `json:"text"`
	BM25Score  float64 `json:"bm25Score"`
	DistanceKm float64 `json:"distanceKm"`
	Lat        float64 `json:"lat"`
	Long       float64 `json:"long"`
}

// Split the text into tokens
func getTokenizedText(text string) []string {
	tokens := strings.Fields(text)
	return tokens
}
