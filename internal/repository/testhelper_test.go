package repository_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://instasae:instasae_dev@localhost:5435/instasae?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Skip("postgres unavailable, skipping")
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skip("postgres unavailable, skipping")
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

func cleanDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, "TRUNCATE conversations, contacts, accounts CASCADE")
	if err != nil {
		t.Fatalf("cleanDB: %v", err)
	}
}
