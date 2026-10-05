package reset

import (
	"barber-booking/internal/testdb"
	"barber-booking/session"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestPostgresLoginAndResetSerialization(t *testing.T) {
	for _, first := range []string{"login", "reset"} {
		t.Run(first, func(t *testing.T) {
			url := testdb.New(t, "../supabase/schema.sql")
			conn, err := sql.Open("pgx", url)
			if err != nil {
				t.Fatal(err)
			}
			ConfigureDatabase(conn)
			session.ConfigureDatabase(conn)
			t.Cleanup(func() { ConfigureDatabase(nil); session.ConfigureDatabase(nil); conn.Close() })
			var id int
			if err := conn.QueryRow("insert into public.customers(full_name,phone_number,email,password_hash) values('Race','123','race@example.com','oldhash') returning id").Scan(&id); err != nil {
				t.Fatal(err)
			}
			token, err := NewContext(context.Background(), session.RoleCustomer, id)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			control, err := conn.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer control.Close()
			key := time.Now().UnixNano()%1000000000 + 1000000000
			if _, err := control.ExecContext(ctx, "select pg_advisory_lock($1)", key); err != nil {
				t.Fatal(err)
			}
			defer control.ExecContext(context.Background(), "select pg_advisory_unlock($1)", key)
			event, table := "insert", "degosalon_internal.auth_sessions"
			if first == "reset" {
				event, table = "delete", "degosalon_internal.password_resets"
			}
			statement := fmt.Sprintf(`create function degosalon_internal.pause_auth() returns trigger language plpgsql as $$ begin perform pg_advisory_xact_lock(%d); if TG_OP='DELETE' then return old; end if; return new; end $$; create trigger pause_auth before %s on %s for each row execute function degosalon_internal.pause_auth()`, key, event, table)
			if _, err := conn.ExecContext(ctx, statement); err != nil {
				t.Fatal(err)
			}
			type loginResult struct {
				s   session.Session
				err error
			}
			login := make(chan loginResult, 1)
			recovery := make(chan error, 1)
			startLogin := func() {
				go func() {
					s, err := session.NewVerifiedContext(ctx, session.RoleCustomer, id, "oldhash", nil)
					login <- loginResult{s, err}
				}()
			}
			startReset := func() {
				go func() { _, err := CompleteContext(ctx, session.RoleCustomer, token, "newhash", nil); recovery <- err }()
			}
			if first == "login" {
				startLogin()
			} else {
				startReset()
			}
			waitLock := func(query string, args ...any) {
				t.Helper()
				deadline := time.Now().Add(2 * time.Second)
				for time.Now().Before(deadline) {
					var waiting bool
					if err := conn.QueryRowContext(ctx, query, args...).Scan(&waiting); err != nil {
						t.Fatal(err)
					}
					if waiting {
						return
					}
					time.Sleep(5 * time.Millisecond)
				}
				t.Fatal("expected transaction did not reach its lock")
			}
			waitLock("select exists(select 1 from pg_locks where locktype='advisory' and objid=$1 and not granted)", key)
			if first == "login" {
				startReset()
				waitLock("select exists(select 1 from pg_stat_activity where datname=current_database() and wait_event_type='Lock' and query like 'update public.customers set password_hash%')")
			} else {
				startLogin()
				waitLock("select exists(select 1 from pg_stat_activity where datname=current_database() and wait_event_type='Lock' and query like 'select password_hash from public.customers%')")
			}
			if _, err := control.ExecContext(ctx, "select pg_advisory_unlock($1)", key); err != nil {
				t.Fatal(err)
			}
			result := <-login
			if err := <-recovery; err != nil {
				t.Fatalf("reset: %v", err)
			}
			if first == "login" {
				if result.err != nil {
					t.Fatal(result.err)
				}
				if _, err := session.GetContext(ctx, result.s.ID); !errors.Is(err, session.ErrNotFound) {
					t.Fatalf("pre-reset login survived: %v", err)
				}
			} else {
				if !errors.Is(result.err, session.ErrCredentialsChanged) {
					t.Fatalf("stale login created after reset: %v", result.err)
				}
			}
			var count int
			if err := conn.QueryRowContext(ctx, "select count(*) from degosalon_internal.auth_sessions where role='customer' and user_id=$1", id).Scan(&count); err != nil || count != 0 {
				t.Fatalf("stale sessions=%d %v", count, err)
			}
		})
	}
}

func TestMemoryLoginCreatedDuringResetIsRevoked(t *testing.T) {
	ConfigureDatabase(nil)
	session.ConfigureDatabase(nil)
	Clear()
	t.Cleanup(Clear)
	token, err := New(session.RoleCustomer, 15)
	if err != nil {
		t.Fatal(err)
	}
	checked, release, written := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	type loginResult struct {
		s   session.Session
		err error
	}
	login := make(chan loginResult, 1)
	recovery := make(chan error, 1)
	go func() {
		s, err := session.NewVerifiedContext(context.Background(), session.RoleCustomer, 15, "oldhash", func(context.Context) error { close(checked); <-release; return nil })
		login <- loginResult{s, err}
	}()
	select {
	case <-checked:
	case <-time.After(time.Second):
		t.Fatal("login did not check credentials")
	}
	go func() {
		_, err := CompleteContext(context.Background(), session.RoleCustomer, token, "newhash", func(context.Context, int, string) error { close(written); return nil })
		recovery <- err
	}()
	select {
	case <-written:
	case <-time.After(time.Second):
		t.Fatal("reset did not write password")
	}
	unblock()
	result := <-login
	if result.err != nil {
		t.Fatal(result.err)
	}
	if err := <-recovery; err != nil {
		t.Fatal(err)
	}
	if _, err := session.GetContext(context.Background(), result.s.ID); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("concurrent login escaped revocation: %v", err)
	}
}
