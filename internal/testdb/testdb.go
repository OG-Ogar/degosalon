// Package testdb creates a disposable database for PostgreSQL integration tests.
package testdb

import (
	"barber-booking/internal/dbmigrate"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// New applies schemaPath only to a newly created database. The configured URL
// is used to create/drop that database; its existing tables are never changed.
func New(t testing.TB, schemaPath string) string {
	t.Helper()
	configured := os.Getenv("TEST_DATABASE_URL")
	if configured == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	address, err := url.Parse(configured)
	if err != nil || (address.Scheme != "postgres" && address.Scheme != "postgresql") {
		t.Fatal("TEST_DATABASE_URL must be a PostgreSQL URL")
	}
	admin, err := sql.Open("pgx", configured)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := admin.PingContext(ctx); err != nil {
		t.Fatalf("connect to test database server: %v", err)
	}
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		t.Fatal(err)
	}
	name := "degosalon_test_" + hex.EncodeToString(random)
	identifier := pgx.Identifier{name}.Sanitize()
	if _, err := admin.ExecContext(ctx, "create database "+identifier); err != nil {
		t.Fatalf("create disposable test database: %v", err)
	}
	t.Cleanup(func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := admin.ExecContext(cleanupContext, "drop database "+identifier+" with (force)"); err != nil {
			t.Errorf("drop disposable test database: %v", err)
		}
	})
	address.Path = "/" + name
	address.RawPath = ""
	databaseURL := address.String()
	connection, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if schemaPath != "" {
		schema, err := os.ReadFile(schemaPath)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := connection.ExecContext(ctx, string(schema)); err != nil {
			t.Fatalf("apply test schema: %v", err)
		}
		if err := dbmigrate.Apply(ctx, connection); err != nil {
			t.Fatalf("apply test migrations: %v", err)
		}
	}
	return databaseURL
}
