package main

import (
	"strings"
)

type doc struct {
	id   int
	text string
}

type scorePair struct {
	Id    int     `json:"id"`
	Score float64 `json:"score"`
	Text  string  `json:"text"`
}

type scoreResult struct {
	id    int
	score float64
}

func getTokenizedText(text string) []string {
	// Split the text into tokens (words)
	tokens := strings.Fields(text)
	return tokens
}
