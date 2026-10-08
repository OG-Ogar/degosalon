package storage

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestReadinessRejectsMissingProductionStorageAndOutages(t *testing.T) {
	previous := db
	db = nil
	t.Cleanup(func() { db = previous })
	t.Setenv("APP_ENV", "test")
	t.Setenv("RENDER", "")
	if err := CheckReadiness(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_ENV", "production")
	if err := CheckReadiness(context.Background()); err == nil {
		t.Fatal("production reported ready without persistence")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := CheckReadiness(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("ignored cancellation: %v", err)
	}
	connection, err := sql.Open("pgx", "postgres://unused:unused@localhost/unused")
	if err != nil {
		t.Fatal(err)
	}
	_ = connection.Close()
	db = connection
	if err := CheckReadiness(context.Background()); err == nil {
		t.Fatal("closed database reported ready")
	}
}
