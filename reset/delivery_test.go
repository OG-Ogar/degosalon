package reset

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"barber-booking/internal/testdb"
	"barber-booking/session"
)

func advanceDelivery(t *testing.T, conn *sql.DB) {
	t.Helper()
	if conn != nil {
		if _, err := conn.Exec("update degosalon_internal.reset_mail_jobs set next_attempt_at=now()-interval '1 second',lease_until=case when lease_key='' then null else now()-interval '1 second' end"); err != nil {
			t.Fatal(err)
		}
		return
	}
	mu.Lock()
	defer mu.Unlock()
	for digest, job := range deliveries {
		job.next = time.Now().Add(-time.Second)
		job.leaseUntil = time.Now().Add(-time.Second)
		deliveries[digest] = job
	}
}

func checkDelivery(t *testing.T, conn *sql.DB, id int) {
	t.Helper()
	ctx := context.Background()
	key := bytes.Repeat([]byte{42}, 32)
	recipient := "recovery_delivery@example.com"
	token, err := NewDeliveryContext(ctx, session.RoleCustomer, id, recipient, key)
	if err != nil {
		t.Fatal(err)
	}
	job, found, err := claimDelivery(ctx)
	if err != nil || !found {
		t.Fatalf("claim: %v %v", found, err)
	}
	if bytes.Contains(job.payload, []byte(token)) || bytes.Contains(job.payload, []byte(recipient)) {
		t.Fatal("private reset payload persisted as plaintext")
	}
	if _, err := job.open(bytes.Repeat([]byte{43}, 32)); !errors.Is(err, ErrDeliveryPayload) {
		t.Fatalf("wrong key: %v", err)
	}
	tampered := job
	tampered.payload = append([]byte(nil), job.payload...)
	tampered.payload[len(tampered.payload)-1] ^= 1
	if _, err := tampered.open(key); !errors.Is(err, ErrDeliveryPayload) {
		t.Fatal("modified ciphertext accepted")
	}
	if _, found, err := claimDelivery(ctx); err != nil || found {
		t.Fatalf("active lease claimed twice: %v %v", found, err)
	}
	advanceDelivery(t, conn)
	newLease, found, err := claimDelivery(ctx)
	if err != nil || !found || newLease.lease == job.lease {
		t.Fatal("crashed lease was not recovered")
	}
	if err := finishDelivery(ctx, job, true); err != nil {
		t.Fatal(err)
	}
	if err := finishDelivery(ctx, newLease, false); err != nil {
		t.Fatal(err)
	}
	if _, found, err := claimDelivery(ctx); err != nil || found {
		t.Fatal("failed delivery retried without backoff")
	}
	advanceDelivery(t, conn)
	var count atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := ProcessDelivery(ctx, key, func(_ context.Context, to, role, gotToken string) error {
				if to != recipient || role != "customer" || gotToken != token {
					return errors.New("incorrect message")
				}
				count.Add(1)
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if count.Load() != 1 {
		t.Fatalf("concurrent sends: %d", count.Load())
	}
	if _, found, err := claimDelivery(ctx); err != nil || found {
		t.Fatal("accepted message was retained")
	}
	if _, err := LookupContext(ctx, session.RoleCustomer, token); err != nil {
		t.Fatal("mail acknowledgment consumed reset token")
	}
	old, err := NewDeliveryContext(ctx, session.RoleCustomer, id, recipient, key)
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := NewDeliveryContext(ctx, session.RoleCustomer, id, recipient, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LookupContext(ctx, session.RoleCustomer, old); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("replaced token valid")
	}
	_, err = ProcessDelivery(ctx, key, func(_ context.Context, _, _, got string) error {
		if got != replacement {
			return errors.New("obsolete message sent")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := NewDeliveryContext(cancelled, session.RoleCustomer, id, recipient, key); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled issuance accepted")
	}
	if _, err := LookupContext(ctx, session.RoleCustomer, replacement); err != nil {
		t.Fatal("failed issuance consumed existing token")
	}
	if _, err := NewDeliveryContext(ctx, session.RoleCustomer, id, recipient, key); err != nil {
		t.Fatal(err)
	}
	relayFailure := errors.New("relay unavailable")
	for i := 0; i < 8; i++ {
		found, err := ProcessDelivery(ctx, key, func(context.Context, string, string, string) error { return relayFailure })
		if !found || !errors.Is(err, relayFailure) {
			t.Fatalf("retry %d: %v %v", i, found, err)
		}
		advanceDelivery(t, conn)
	}
	if found, err := ProcessDelivery(ctx, key, func(context.Context, string, string, string) error { t.Error("retry limit bypassed"); return nil }); found || err != nil {
		t.Fatal("exhausted job remained eligible")
	}
}

func TestEncryptedDeliveryLeasesAndRetry(t *testing.T) {
	ConfigureDatabase(nil)
	Clear()
	t.Cleanup(Clear)
	checkDelivery(t, nil, 1)
}

func TestPostgresEncryptedDeliveryPersistenceAndAtomicity(t *testing.T) {
	databaseURL := testdb.New(t, "../supabase/schema.sql")
	conn, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	ConfigureDatabase(conn)
	t.Cleanup(func() { ConfigureDatabase(nil); conn.Close() })
	var id int
	if err := conn.QueryRow("insert into public.customers(full_name,phone_number,email,password_hash) values('Delivery','123','delivery@example.com','oldhash') returning id").Scan(&id); err != nil {
		t.Fatal(err)
	}
	checkDelivery(t, conn, id)
	key := bytes.Repeat([]byte{42}, 32)
	token, err := NewDeliveryContext(context.Background(), session.RoleCustomer, id, "delivery@example.com", key)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	conn, err = sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	ConfigureDatabase(conn)
	_, err = ProcessDelivery(context.Background(), key, func(_ context.Context, _, _, got string) error {
		if got != token {
			return errors.New("restart changed link")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`create function degosalon_internal.reject_delivery() returns trigger language plpgsql as $$ begin raise exception 'delivery insert failed'; end $$;
	create trigger reject_delivery before insert on degosalon_internal.reset_mail_jobs for each row execute function degosalon_internal.reject_delivery()`); err != nil {
		t.Fatal(err)
	}
	if _, err := NewDeliveryContext(context.Background(), session.RoleCustomer, id, "delivery@example.com", key); err == nil {
		t.Fatal("failed enqueue reported acceptance")
	}
	if _, err := LookupContext(context.Background(), session.RoleCustomer, token); err != nil {
		t.Fatal("enqueue failure replaced previously valid token")
	}
	if _, err := conn.Exec("drop trigger reject_delivery on degosalon_internal.reset_mail_jobs"); err != nil {
		t.Fatal(err)
	}
	if _, err := NewDeliveryContext(context.Background(), session.RoleCustomer, id, "delivery@example.com", key); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec("update degosalon_internal.password_resets set expires_at=now()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if _, found, err := claimDelivery(context.Background()); err != nil || found {
		t.Fatal("expired message claimed")
	}
	var remaining int
	if err := conn.QueryRow("select count(*) from degosalon_internal.reset_mail_jobs").Scan(&remaining); err != nil || remaining != 0 {
		t.Fatal("expired encrypted payload retained")
	}
	conn.Close()
	if _, _, err := claimDelivery(context.Background()); err == nil {
		t.Fatal("delivery query outage fell back to memory")
	}
}
