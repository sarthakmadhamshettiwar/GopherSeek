package main

import (
	"strings"
)

type doc struct {
	id   int
	text string
}

type scorePair struct {
	id    int
	score float64
	text  string
}


func getTokenizedText(text string) []string {
	// Split the text into tokens (words)
	tokens := strings.Fields(text)
	return tokens
}
