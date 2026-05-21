package data

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func GetCorpusFromFile() []Doc {
	file, err := os.ReadFile("data/corpus.json")
	if err != nil {
		slog.Error("failed to read corpus file", "path", "data/corpus.json", "error", err)
		return nil
	}

	var raw []map[string]interface{}
	if err := json.Unmarshal(file, &raw); err != nil {
		slog.Error("failed to parse corpus JSON", "error", err)
		return nil
	}

	var corpus []Doc
	for _, item := range raw {
		var d Doc
		if idv, ok := item["id"]; ok {
			switch v := idv.(type) {
			case float64:
				d.Id = int(v)
			case int:
				d.Id = v
			}
		}
		if v, ok := item["name"].(string); ok {
			d.Name = v
		}
		// "description" in JSON → Doc.Text for BM25
		if v, ok := item["description"].(string); ok {
			d.Text = v
		}
		if v, ok := item["lat"].(float64); ok {
			d.Lat = v
		}
		if v, ok := item["long"].(float64); ok {
			d.Long = v
		}
		corpus = append(corpus, d)
	}
	slog.Info("loaded corpus from file", "docs", len(corpus))
	return corpus
}

func loadDBConfig() (context.Context, context.CancelFunc, string, error) {
	if err := godotenv.Load(); err != nil {
		slog.Warn("no .env file found, falling back to environment", "error", err)
	}
	connString := os.Getenv("DOCS_DATABASE_URL")
	if connString == "" {
		slog.Error("DOCS_DATABASE_URL not set")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	return ctx, cancel, connString, nil
}

func scanCorpusRows(rows pgx.Rows) []Doc {
	defer rows.Close()
	var corpus []Doc
	for rows.Next() {
		var d Doc
		if err := rows.Scan(&d.Id, &d.Name, &d.Text, &d.Lat, &d.Long); err != nil {
			slog.Error("failed to scan row", "error", err)
			return []Doc{}
		}
		corpus = append(corpus, d)
	}
	return corpus
}

func GetCorpusFromDB() []Doc {
	ctx, cancel, connString, err := loadDBConfig()
	if err != nil {
		slog.Error("failed to load DB config", "error", err)
		return []Doc{}
	}
	defer cancel()

	slog.Info("connecting to database")
	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		slog.Error("unable to connect to database", "error", err)
		return []Doc{}
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, "SELECT id, name, text, lat, long FROM docs;")
	if err != nil {
		slog.Error("failed to query docs", "error", err)
		return []Doc{}
	}
	corpus := scanCorpusRows(rows)
	slog.Info("loaded corpus from database", "docs", len(corpus))
	return corpus
}

func GetCorpusFromDBPool() []Doc {
	ctx, cancel, connString, err := loadDBConfig()
	if err != nil {
		slog.Error("failed to load DB config", "error", err)
		return []Doc{}
	}
	defer cancel()

	slog.Info("connecting to database via pool")
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		slog.Error("unable to create connection pool", "error", err)
		return []Doc{}
	}
	defer pool.Close()

	rows, err := pool.Query(ctx, "SELECT id, name, text, lat, long FROM docs;")
	if err != nil {
		slog.Error("failed to query docs", "error", err)
		return []Doc{}
	}
	corpus := scanCorpusRows(rows)
	slog.Info("loaded corpus from database pool", "docs", len(corpus))
	return corpus
}

func GetCorpus(source string) []Doc {
	slog.Info("fetching corpus", "source", source)
	switch source {
	case "db":
		if os.Getenv("USE_POOL") == "true" {
			return GetCorpusFromDBPool()
		}
		return GetCorpusFromDB()
	case "file":
		return GetCorpusFromFile()
	}
	slog.Error("unknown corpus source", "source", source)
	return nil
}
