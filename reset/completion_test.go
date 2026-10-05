package reset

import (
	"barber-booking/session"
	"context"
	"errors"
	"sync"
	"testing"
)

func TestCompletionRetainsFailedTokenAndRevokesSessions(t *testing.T) {
	ConfigureDatabase(nil)
	session.ConfigureDatabase(nil)
	Clear()
	t.Cleanup(Clear)
	token, err := New(session.RoleCustomer, 9)
	if err != nil {
		t.Fatal(err)
	}
	s, err := session.New(session.RoleCustomer, 9)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Delete(s.ID) })
	failure := errors.New("write failed")
	if _, err := CompleteContext(context.Background(), session.RoleCustomer, token, "newhash", func(context.Context, int, string) error { return failure }); !errors.Is(err, failure) {
		t.Fatalf("write failure: %v", err)
	}
	if _, ok := Lookup(session.RoleCustomer, token); !ok {
		t.Fatal("failed write consumed token")
	}
	if _, ok := session.Get(s.ID); !ok {
		t.Fatal("failed write revoked session")
	}
	var writes int
	var writeMu sync.Mutex
	apply := func(_ context.Context, id int, hash string) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		if id != 9 || hash != "newhash" {
			t.Fatal("wrong account/hash")
		}
		writes++
		return nil
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			_, err := CompleteContext(context.Background(), session.RoleCustomer, token, "newhash", apply)
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
	if success != 1 || writes != 1 {
		t.Fatalf("completion race: successes=%d writes=%d", success, writes)
	}
	if _, ok := Lookup(session.RoleCustomer, token); ok {
		t.Fatal("consumed token remains")
	}
	if _, ok := session.Get(s.ID); ok {
		t.Fatal("old session remains")
	}
	token, err = New(session.RoleBarber, 9)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := LookupContext(ctx, session.RoleBarber, token); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled lookup: %v", err)
	}
	if _, err := CompleteContext(ctx, session.RoleBarber, token, "hash", apply); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled completion: %v", err)
	}
	if _, ok := Lookup(session.RoleBarber, token); !ok {
		t.Fatal("cancelled request consumed token")
	}
}
