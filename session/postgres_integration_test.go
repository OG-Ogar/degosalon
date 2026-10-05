package session

import (
	"barber-booking/internal/testdb"
	"context"
	"database/sql"
	"errors"
	_ "github.com/jackc/pgx/v5/stdlib"
	"net/http/httptest"
	"testing"
)

func TestPostgresSessionPersistence(t *testing.T) {
	url := testdb.New(t, "../supabase/schema.sql")
	first, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	ConfigureDatabase(first)
	t.Cleanup(func() { ConfigureDatabase(nil); first.Close() })
	ctx := context.Background()
	s, err := NewContext(ctx, RoleCustomer, 7)
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewContext(ctx, RoleBarber, 7)
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := first.QueryRow("select id_digest from degosalon_internal.auth_sessions where role='customer'").Scan(&stored); err != nil || stored != digest(s.ID) || stored == s.ID {
		t.Fatalf("stored digest: %s %v", stored, err)
	}
	first.Close()
	second, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { second.Close() })
	ConfigureDatabase(second)
	recovered, err := GetContext(ctx, s.ID)
	if err != nil || recovered.UserID != 7 || recovered.Role != s.Role || recovered.CSRFToken != s.CSRFToken {
		t.Fatalf("restart persistence: %+v %v", recovered, err)
	}
	if _, err := second.Exec("alter table degosalon_internal.auth_sessions rename to unavailable_sessions"); err != nil {
		t.Fatal(err)
	}
	if _, err := GetContext(ctx, s.ID); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("query failure hidden: %v", err)
	}
	w := httptest.NewRecorder()
	if _, err := StartContext(ctx, w, RoleCustomer, 7); err == nil || len(w.Result().Cookies()) != 0 {
		t.Fatal("failed persistence issued a cookie")
	}
	if err := DeleteForUserContext(ctx, RoleCustomer, 7); err == nil {
		t.Fatal("failed revocation hidden")
	}
	if _, err := second.Exec("alter table degosalon_internal.unavailable_sessions rename to auth_sessions"); err != nil {
		t.Fatal(err)
	}
	if _, err := GetContext(ctx, s.ID); err != nil {
		t.Fatalf("session lost during outage: %v", err)
	}
	if err := DeleteForUserContext(ctx, RoleCustomer, 7); err != nil {
		t.Fatal(err)
	}
	if _, err := GetContext(ctx, s.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revocation: %v", err)
	}
	if _, err := GetContext(ctx, other.ID); err != nil {
		t.Fatalf("cross-role revocation: %v", err)
	}
	if _, err := second.Exec("update degosalon_internal.auth_sessions set expires_at=now()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if _, err := GetContext(ctx, other.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expiry: %v", err)
	}
	if _, err := NewContext(ctx, RoleAdmin, 0); err != nil {
		t.Fatal(err)
	}
	var expired int
	if err := second.QueryRow("select count(*) from degosalon_internal.auth_sessions where expires_at<=now()").Scan(&expired); err != nil || expired != 0 {
		t.Fatalf("bounded expiry cleanup: %d %v", expired, err)
	}
}
