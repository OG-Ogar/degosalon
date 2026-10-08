package storage

import (
	"barber-booking/internal/testdb"
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

func checkAdminAudit(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	id, err := BeginAdminAudit(ctx, "operator", "barber.archive", 42, strings.Repeat("a", 32))
	if err != nil {
		t.Fatal(err)
	}
	if err := FinishAdminAudit(ctx, id, 409, 0); err != nil {
		t.Fatal(err)
	}
	unfinished, err := BeginAdminAudit(ctx, "operator", "site.landing_image", 0, strings.Repeat("b", 32))
	if err != nil {
		t.Fatal(err)
	}
	events, err := ListAdminAudit(ctx, 0, 2)
	if err != nil || len(events) != 2 {
		t.Fatalf("audit list: %v %v", events, err)
	}
	if events[0].ID != unfinished || events[0].Phase != "started" || events[1].HTTPStatus != 409 || events[1].RequestID != strings.Repeat("a", 32) {
		t.Fatalf("wrong audit outcome: %v", events)
	}
	older, err := ListAdminAudit(ctx, events[1].ID, 2)
	if err != nil || len(older) != 1 || older[0].ID != id || older[0].Phase != "started" {
		t.Fatalf("wrong audit page: %v %v", older, err)
	}
	if _, err := BeginAdminAudit(ctx, "operator", "barber.archive", 42, strings.Repeat("a", 32)); err == nil {
		t.Fatal("duplicate audit request accepted")
	}
	if err := FinishAdminAudit(ctx, id, 200, 0); err == nil {
		t.Fatal("duplicate completion accepted")
	}
	if err := FinishAdminAudit(ctx, 999999, 200, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing attempt: %v", err)
	}
	created, err := BeginAdminAudit(ctx, "operator", "barber.create", 0, strings.Repeat("d", 32))
	if err != nil {
		t.Fatal(err)
	}
	if err := FinishAdminAudit(ctx, created, 303, 123); err != nil {
		t.Fatal(err)
	}
	latest, err := ListAdminAudit(ctx, 0, 1)
	if err != nil || latest[0].TargetID != 123 {
		t.Fatalf("created account missing: %v %v", latest, err)
	}

	if _, err := BeginAdminAudit(ctx, "operator", "arbitrary-secret", 42, strings.Repeat("c", 32)); err == nil {
		t.Fatal("unbounded action accepted")
	}
	if _, err := ListAdminAudit(ctx, 0, 201); err == nil {
		t.Fatal("unbounded audit query")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := BeginAdminAudit(canceled, "operator", "barber.update", 42, strings.Repeat("c", 32)); !errors.Is(err, context.Canceled) {
		t.Fatalf("ignored cancellation: %v", err)
	}
}
func TestAdminAuditMemory(t *testing.T) { Reset(); t.Cleanup(Reset); checkAdminAudit(t) }
func TestPostgresAdminAuditPersistenceAndImmutability(t *testing.T) {
	address := testdb.New(t, "../supabase/schema.sql")
	connection, err := sql.Open("pgx", address)
	if err != nil {
		t.Fatal(err)
	}
	previous := db
	db = connection
	t.Cleanup(func() { db = previous; connection.Close() })
	checkAdminAudit(t)
	if _, err := db.Exec("update degosalon_internal.admin_audit_events set actor='forged'"); err == nil {
		t.Fatal("audit rewrite accepted")
	}
	if _, err := db.Exec("delete from degosalon_internal.admin_audit_events"); err == nil {
		t.Fatal("audit deletion accepted")
	}
	reopened, err := sql.Open("pgx", address)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	db = reopened
	events, err := ListAdminAudit(context.Background(), 0, 100)
	if err != nil || len(events) != 5 {
		t.Fatalf("audit did not survive reconnect: %v %v", events, err)
	}
}
