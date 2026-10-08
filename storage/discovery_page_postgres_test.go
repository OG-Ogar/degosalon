package storage

import (
	"barber-booking/internal/testdb"
	"context"
	"database/sql"
	"encoding/json"
	"testing"
)

func TestPostgresDiscoveryPages(t *testing.T) {
	address := testdb.New(t, "../supabase/schema.sql")
	connection, err := sql.Open("pgx", address)
	if err != nil {
		t.Fatal(err)
	}
	previous := db
	db = connection
	t.Cleanup(func() { db = previous; connection.Close() })
	checkDiscoveryPages(t)
	_, _, query, args := discoveryStatements(DiscoveryFilter{StateID: "NG025", AreaID: "NG025011", Limit: 24})
	var plan []byte
	if err := db.QueryRowContext(context.Background(), "explain (analyze,buffers,format json) "+query, args...).Scan(&plan); err != nil {
		t.Fatal(err)
	}
	var parsed any
	if err := json.Unmarshal(plan, &parsed); err != nil {
		t.Fatal(err)
	}
	t.Logf("Page-scoped discovery query plan: %s", plan)
}
