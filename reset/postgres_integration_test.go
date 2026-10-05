package reset

import (
	"barber-booking/internal/testdb"
	"barber-booking/session"
	"context"
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
	"testing"
)

func TestPostgresRecoveryPersistenceAndAtomicity(t *testing.T) {
	url := testdb.New(t, "../supabase/schema.sql")
	conn, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	ConfigureDatabase(conn)
	session.ConfigureDatabase(conn)
	t.Cleanup(func() { ConfigureDatabase(nil); session.ConfigureDatabase(nil); conn.Close() })
	ctx := context.Background()
	var id int
	if err := conn.QueryRow("insert into public.customers(full_name,phone_number,email,password_hash) values('Recovery','123','recovery@example.com','oldhash') returning id").Scan(&id); err != nil {
		t.Fatal(err)
	}
	token, err := NewContext(ctx, session.RoleCustomer, id)
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := conn.QueryRow("select token_digest from degosalon_internal.password_resets").Scan(&stored); err != nil || stored != fingerprint(token) || stored == token {
		t.Fatalf("stored token: %s %v", stored, err)
	}
	conn.Close()
	conn, err = sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	ConfigureDatabase(conn)
	session.ConfigureDatabase(conn)
	if got, err := LookupContext(ctx, session.RoleCustomer, token); err != nil || got != id {
		t.Fatalf("reconnect: %d %v", got, err)
	}
	replacement, err := NewContext(ctx, session.RoleCustomer, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LookupContext(ctx, session.RoleCustomer, token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("replacement: %v", err)
	}
	token = replacement
	if _, err := LookupContext(ctx, session.RoleBarber, token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("wrong role: %v", err)
	}
	s, err := session.New(session.RoleCustomer, id)
	if err != nil {
		t.Fatal(err)
	}
	for i, table := range []string{"public.customers", "degosalon_internal.auth_sessions", "degosalon_internal.password_resets"} {
		t.Run(table, func(t *testing.T) {
			function := fmt.Sprintf("fail_recovery_%d", i)
			trigger := function + "_trigger"
			event := "delete"
			if i == 0 {
				event = "update"
			}
			if _, err := conn.Exec(fmt.Sprintf(`create function degosalon_internal.%s() returns trigger language plpgsql as $$ begin raise exception 'injected recovery failure'; end $$; create trigger %s before %s on %s for each row execute function degosalon_internal.%s()`, function, trigger, event, table, function)); err != nil {
				t.Fatal(err)
			}
			cleanup := func() {
				if _, err := conn.Exec(fmt.Sprintf("drop trigger if exists %s on %s; drop function if exists degosalon_internal.%s()", trigger, table, function)); err != nil {
					t.Error(err)
				}
			}
			t.Cleanup(cleanup)
			if _, err := CompleteContext(ctx, session.RoleCustomer, token, "newhash", nil); err == nil {
				t.Fatal("injected completion succeeded")
			}
			var hash string
			if err := conn.QueryRow("select password_hash from public.customers where id=$1", id).Scan(&hash); err != nil || hash != "oldhash" {
				t.Fatalf("password rollback: %s %v", hash, err)
			}
			if _, err := LookupContext(ctx, session.RoleCustomer, token); err != nil {
				t.Fatalf("token rollback: %v", err)
			}
			if _, err := session.GetContext(ctx, s.ID); err != nil {
				t.Fatalf("session rollback: %v", err)
			}
			cleanup()
		})
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			_, err := CompleteContext(ctx, session.RoleCustomer, token, "newhash", nil)
			results <- err
		}()
	}
	close(start)
	success := 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			success++
		} else if !errors.Is(err, ErrInvalidToken) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("concurrent completion successes=%d", success)
	}
	var hash string
	if err := conn.QueryRow("select password_hash from public.customers where id=$1", id).Scan(&hash); err != nil || hash != "newhash" {
		t.Fatalf("password: %s %v", hash, err)
	}
	if _, err := LookupContext(ctx, session.RoleCustomer, token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("consumed token: %v", err)
	}
	if _, err := session.GetContext(ctx, s.ID); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("revoked session: %v", err)
	}
	token, err = NewContext(ctx, session.RoleCustomer, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("alter table degosalon_internal.password_resets rename to unavailable_resets"); err != nil {
		t.Fatal(err)
	}
	if _, err := LookupContext(ctx, session.RoleCustomer, token); err == nil || errors.Is(err, ErrInvalidToken) {
		t.Fatalf("lookup failure hidden: %v", err)
	}
	if _, err := NewContext(ctx, session.RoleCustomer, id); err == nil {
		t.Fatal("issuance failure hidden")
	}
	if _, err := conn.Exec("alter table degosalon_internal.unavailable_resets rename to password_resets"); err != nil {
		t.Fatal(err)
	}
	if _, err := LookupContext(ctx, session.RoleCustomer, token); err != nil {
		t.Fatalf("outage recovery: %v", err)
	}
	if _, err := conn.Exec("update degosalon_internal.password_resets set expires_at=now()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if _, err := LookupContext(ctx, session.RoleCustomer, token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expiry: %v", err)
	}
}
