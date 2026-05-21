package application

import "strings"

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
	RadiusKm float64
	TopK     int
	SortBy   string
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

func getTokenizedText(text string) []string {
	return strings.Fields(text)
}
