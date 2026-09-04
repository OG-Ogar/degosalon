package reset

import (
	"testing"
	"time"

	"barber-booking/session"
)

// mustNew issues a token or ends the test.
func mustNew(t *testing.T, role session.Role, userID int) string {
	t.Helper()

	token, err := New(role, userID)
	if err != nil {
		t.Fatalf("New(%s, %d): %v", role, userID, err)
	}

	return token
}

// TestTokenIsSingleUse is the property the whole flow rests on: a link
// that has already set a password must not set another.
func TestTokenIsSingleUse(t *testing.T) {
	Clear()

	token := mustNew(t, session.RoleCustomer, 7)

	id, ok := Consume(session.RoleCustomer, token)
	if !ok || id != 7 {
		t.Fatalf("first use returned (%d, %v), want (7, true)", id, ok)
	}

	if _, ok := Consume(session.RoleCustomer, token); ok {
		t.Fatal("the same link was accepted a second time")
	}
}

// TestTokenBelongsToOneRole keeps a customer's link from being replayed
// against the barber form, where it would name a different account.
func TestTokenBelongsToOneRole(t *testing.T) {
	Clear()

	token := mustNew(t, session.RoleCustomer, 1)

	if _, ok := Lookup(session.RoleBarber, token); ok {
		t.Fatal("a customer's link was accepted as a barber's")
	}

	if _, ok := Lookup(session.RoleCustomer, token); !ok {
		t.Fatal("the link stopped working for the role it was issued to")
	}
}

// TestNewTokenReplacesTheOldOne checks that asking twice leaves exactly
// one working link, which is also what bounds the map.
func TestNewTokenReplacesTheOldOne(t *testing.T) {
	Clear()

	first := mustNew(t, session.RoleCustomer, 3)
	second := mustNew(t, session.RoleCustomer, 3)

	if _, ok := Lookup(session.RoleCustomer, first); ok {
		t.Fatal("the earlier link still works")
	}

	if _, ok := Lookup(session.RoleCustomer, second); !ok {
		t.Fatal("the newest link does not work")
	}

	if len(pending) != 1 {
		t.Fatalf("holding %d tokens for one account, want 1", len(pending))
	}
}

// TestExpiredTokenIsRefusedAndDropped ages the stored entry by hand.
// Lifetime is a constant, so the alternative would be a test that waits
// half an hour.
func TestExpiredTokenIsRefusedAndDropped(t *testing.T) {
	Clear()

	token := mustNew(t, session.RoleBarber, 2)
	digest := fingerprint(token)

	aged := pending[digest]
	aged.expiresAt = time.Now().Add(-time.Minute)
	pending[digest] = aged

	if _, ok := Lookup(session.RoleBarber, token); ok {
		t.Fatal("an expired link still works")
	}

	if _, held := pending[digest]; held {
		t.Fatal("the expired token was left behind in the map")
	}
}

// TestEmptyTokenIsRefused covers a form submitted with no token at all,
// which must not resolve to whatever the digest of "" happens to be.
func TestEmptyTokenIsRefused(t *testing.T) {
	Clear()

	mustNew(t, session.RoleCustomer, 4)

	if _, ok := Lookup(session.RoleCustomer, ""); ok {
		t.Fatal("an empty token was accepted")
	}
}
