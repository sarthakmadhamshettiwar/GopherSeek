package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"search/application"
	"search/data"
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

const concurrentQueries = 100

// loadConnStringForBench loads DOCS_DATABASE_URL from .env; skips the benchmark if not set.
func loadConnStringForBench(b *testing.B) string {
	b.Helper()
	// try root .env (go test sets cwd to the package directory)
	_ = godotenv.Load()
	_ = godotenv.Load("../.env")
	connString := os.Getenv("DOCS_DATABASE_URL")
	if connString == "" {
		b.Skip("DOCS_DATABASE_URL not set")
	}
	return connString
}

// BenchmarkDB_DirectConnection spawns 100 goroutines, each opening its own pgx.Connect.
func BenchmarkDB_DirectConnection_100Concurrent(b *testing.B) {
	connString := loadConnStringForBench(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for j := 0; j < concurrentQueries; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				conn, err := pgx.Connect(ctx, connString)
				if err != nil {
					b.Errorf("pgx.Connect: %v", err)
					return
				}
				defer conn.Close(ctx)
				rows, err := conn.Query(ctx, "SELECT id, name, text, lat, long FROM docs;")
				if err != nil {
					b.Errorf("conn.Query: %v", err)
					return
				}
				for rows.Next() {
					var id int
					var name, text string
					var lat, long float64
					rows.Scan(&id, &name, &text, &lat, &long)
				}
				rows.Close()
			}()
		}
		wg.Wait()
	}
}

// BenchmarkDB_SharedPool_100Concurrent creates one pgxpool once, then runs 100 concurrent queries against it.
func BenchmarkDB_SharedPool_100Concurrent(b *testing.B) {
	connString := loadConnStringForBench(b)
	cfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		b.Fatalf("pgxpool.ParseConfig: %v", err)
	}
	cfg.MaxConns = int32(concurrentQueries)
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		b.Fatalf("pgxpool.NewWithConfig: %v", err)
	}
	defer pool.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for j := 0; j < concurrentQueries; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				rows, err := pool.Query(ctx, "SELECT id, name, text, lat, long FROM docs;")
				if err != nil {
					b.Errorf("pool.Query: %v", err)
					return
				}
				for rows.Next() {
					var id int
					var name, text string
					var lat, long float64
					rows.Scan(&id, &name, &text, &lat, &long)
				}
				rows.Close()
			}()
		}
		wg.Wait()
	}
}

// BenchmarkDB_PoolPerCall_100Concurrent mirrors the current data.GetCorpusFromDBPool() implementation —
// each goroutine creates its own pool. This shows the overhead of that pattern.
func BenchmarkDB_PoolPerCall_100Concurrent(b *testing.B) {
	loadConnStringForBench(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for j := 0; j < concurrentQueries; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				data.GetCorpusFromDBPool()
			}()
		}
		wg.Wait()
	}
}

const waves = 5

// runQueryBurst fires concurrentQueries goroutines against the pool and waits for all to finish.
func runQueryBurst(b *testing.B, pool *pgxpool.Pool) {
	b.Helper()
	var wg sync.WaitGroup
	for j := 0; j < concurrentQueries; j++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			rows, err := pool.Query(ctx, "SELECT id, name, text, lat, long FROM docs;")
			if err != nil {
				b.Errorf("pool.Query: %v", err)
				return
			}
			for rows.Next() {
				var id int
				var name, text string
				var lat, long float64
				rows.Scan(&id, &name, &text, &lat, &long)
			}
			rows.Close()
		}()
	}
	wg.Wait()
}

func newPoolWithMaxConns(b *testing.B, connString string, maxConns int32) *pgxpool.Pool {
	b.Helper()
	cfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		b.Fatalf("pgxpool.ParseConfig: %v", err)
	}
	cfg.MaxConns = maxConns
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		b.Fatalf("pgxpool.NewWithConfig: %v", err)
	}
	return pool
}

// BenchmarkDB_DirectConnection_MultiWave fires 5 waves of 100 concurrent queries.
// Each goroutine opens a fresh connection per wave — no reuse across waves.
func BenchmarkDB_DirectConnection_MultiWave(b *testing.B) {
	connString := loadConnStringForBench(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for w := 0; w < waves; w++ {
			var wg sync.WaitGroup
			for j := 0; j < concurrentQueries; j++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					conn, err := pgx.Connect(ctx, connString)
					if err != nil {
						b.Errorf("pgx.Connect: %v", err)
						return
					}
					defer conn.Close(ctx)
					rows, err := conn.Query(ctx, "SELECT id, name, text, lat, long FROM docs;")
					if err != nil {
						b.Errorf("conn.Query: %v", err)
						return
					}
					for rows.Next() {
						var id int
						var name, text string
						var lat, long float64
						rows.Scan(&id, &name, &text, &lat, &long)
					}
					rows.Close()
				}()
			}
			wg.Wait()
		}
	}
}

// BenchmarkDB_SharedPool_MultiWave fires the same 5 waves but through a shared pool.
// Connections established in wave 1 are reused in waves 2–5.
func BenchmarkDB_SharedPool_MultiWave(b *testing.B) {
	connString := loadConnStringForBench(b)
	pool := newPoolWithMaxConns(b, connString, int32(concurrentQueries))
	defer pool.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for w := 0; w < waves; w++ {
			runQueryBurst(b, pool)
		}
	}
}

// BenchmarkDB_SharedPool_Warmed pre-warms all 100 connections before the timer starts,
// then measures the pure steady-state query cost with no handshake overhead.
func BenchmarkDB_SharedPool_Warmed(b *testing.B) {
	connString := loadConnStringForBench(b)
	pool := newPoolWithMaxConns(b, connString, int32(concurrentQueries))
	defer pool.Close()
	runQueryBurst(b, pool) // warm up — not timed
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runQueryBurst(b, pool)
	}
}

func BenchmarkSearchComparison(b *testing.B) {
	sizes := []int{200, 500, 5000, 50000}

	for _, size := range sizes {
		ids, corpus, index, avgLen := setupMockData(size)

		b.Run(fmt.Sprintf("Sequential_Size_%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				application.GetDocumentScoresByIdSequential("gold", ids, corpus, index, avgLen)
			}
		})

		b.Run(fmt.Sprintf("Parallel_Size_%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				application.GetDocumentScoresByIdParallel("gold", ids, corpus, index, avgLen)
			}
		})
	}
}
