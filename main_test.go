package main

import (
	"fmt"
	"testing"
)

func setupMockData(size int) ([]int, map[int][]string, map[string][]int, float64) {
	corpus := make(map[int][]string)
	index := make(map[string][]int)
	ids := make([]int, size)
	for i := 0; i < size; i++ {
		corpus[i] = []string{"this", "is", "a", "gold", "document"}
		index["gold"] = append(index["gold"], i)
		ids[i] = i
	}
	return ids, corpus, index, 5.0
}

func BenchmarkSearchComparison(b *testing.B) {
	sizes := []int{200, 500, 5000, 50000}

	for _, size := range sizes {
		ids, corpus, index, avgLen := setupMockData(size)

		b.Run(fmt.Sprintf("Sequential_Size_%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				getDocumentScoresByIdSequential("gold", ids, corpus, index, avgLen)
			}
		})

		b.Run(fmt.Sprintf("Parallel_Size_%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				getDocumentScoresByIdParallel("gold", ids, corpus, index, avgLen)
			}
		})
	}
}
