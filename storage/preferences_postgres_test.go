package storage

import (
	"context"
	"database/sql"
	"testing"

	"barber-booking/internal/testdb"
	"barber-booking/models"
)

// TestPostgresDisplayPreferences runs against a real disposable database so the
// user_preferences migration, its check constraints and the upsert are
// verified, not just the in-memory mirror.
func TestPostgresDisplayPreferences(t *testing.T) {
	databaseURL := testdb.New(t, "../supabase/schema.sql")
	connection, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	previous := db
	db = connection
	t.Cleanup(func() { db = previous; connection.Close() })
	ctx := context.Background()

	// No row yet: the defaults come back.
	got, err := DisplayPreferencesContext(ctx, "customer", 42)
	if err != nil {
		t.Fatal(err)
	}
	if got != models.DefaultDisplayPreferences() {
		t.Fatalf("absent row = %+v", got)
	}

	want := models.DisplayPreferences{Theme: models.ThemeMidnight, TextSize: models.TextSizeLarge, NotifyAlerts: true}
	if _, err := SaveDisplayPreferencesContext(ctx, "customer", 42, want); err != nil {
		t.Fatal(err)
	}
	got, err = DisplayPreferencesContext(ctx, "customer", 42)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("round trip = %+v", got)
	}

	// A second save updates the row rather than inserting another.
	updated := models.DisplayPreferences{Theme: models.ThemeStone, TextSize: models.TextSizeStandard}
	if _, err := SaveDisplayPreferencesContext(ctx, "customer", 42, updated); err != nil {
		t.Fatal(err)
	}
	got, err = DisplayPreferencesContext(ctx, "customer", 42)
	if err != nil {
		t.Fatal(err)
	}
	if got != updated {
		t.Fatalf("upsert = %+v", got)
	}
	var count int
	if err := connection.QueryRowContext(ctx, "select count(*) from degosalon_internal.user_preferences where user_role='customer' and user_id=42").Scan(&count); err != nil || count != 1 {
		t.Fatalf("row count=%d %v", count, err)
	}

	// A crafted value is normalised before it can reach the check constraints.
	if _, err := SaveDisplayPreferencesContext(ctx, "barber", 9, models.DisplayPreferences{Theme: "<script>", TextSize: "huge"}); err != nil {
		t.Fatal(err)
	}
	stored, err := DisplayPreferencesContext(ctx, "barber", 9)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Theme != models.ThemeForest || stored.TextSize != models.TextSizeStandard {
		t.Fatalf("normalised = %+v", stored)
	}
}
