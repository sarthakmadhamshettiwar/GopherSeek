package main

import (
    "testing"
	"fmt"
)

// Helper to create a dummy dataset for testing
func setupMockData(size int) (map[int][]string, map[string][]int, float64) {
	corpus := make(map[int][]string)
	index := make(map[string][]int)
	for i := 0; i < size; i++ {
		// Mock data: each doc has the word "gold"
		corpus[i] = []string{"this", "is", "a", "gold", "document"}
		index["gold"] = append(index["gold"], i)
	}
	return corpus, index, 5.0
}

func BenchmarkSearchComparison(b *testing.B) { // We test different corpus sizes to see the "break-even" point
	sizes := []int{200, 500, 5000, 50000}

	for _, size := range sizes {
		corpus, index, avgLen := setupMockData(size)

		b.Run(fmt.Sprintf("Sequential_Size_%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				getDocumentScoresByIdSequential("gold", corpus, index, avgLen)
			}
		})

		b.Run(fmt.Sprintf("Parallel_Size_%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				getDocumentScoresByIdParallel("gold", corpus, index, avgLen)
			}
		})
	}
}
