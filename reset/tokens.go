// Package reset issues the single-use tokens behind "I forgot my
// password".
//
// A token is 32 random bytes, handed to the account owner as a link.
// Only its SHA-256 digest is stored, so the contents of this package can
// never be replayed as a working link. Tokens expire, are destroyed on
// first use, and a new request replaces whatever token that account
// already had.
//
// Nothing is written to disk, so outstanding links stop working when the
// process restarts. That is the same trade-off the session package makes.
package reset

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"barber-booking/session"
)

// Lifetime is how long a reset link keeps working.
const Lifetime = 30 * time.Minute

// request is one outstanding reset, stored under its token's digest.
type request struct {
	role      session.Role
	userID    int
	expiresAt time.Time
}

// expired reports whether the link behind this request has run out.
func (r request) expired() bool {
	return time.Now().After(r.expiresAt)
}

var (
	mu      sync.Mutex
	pending = map[string]request{}
)

// New issues a token for one account and returns it in plain form, for
// the caller to put in a link.
//
// Any token the account already had is dropped, so the newest link is
// the only one that works. That also bounds the map: it holds at most
// one entry per account.
func New(role session.Role, userID int) (string, error) {
	buf := make([]byte, 32)

	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}

	token := hex.EncodeToString(buf)

	mu.Lock()
	defer mu.Unlock()

	for digest, existing := range pending {
		if existing.expired() ||
			(existing.role == role && existing.userID == userID) {

			delete(pending, digest)
		}
	}

	pending[fingerprint(token)] = request{
		role:      role,
		userID:    userID,
		expiresAt: time.Now().Add(Lifetime),
	}

	return token, nil
}

// Lookup reports which account a token belongs to and leaves it usable,
// so a link can be checked before the visitor is asked to type anything.
func Lookup(role session.Role, token string) (int, bool) {
	mu.Lock()
	defer mu.Unlock()

	found, ok := resolve(role, token)

	return found.userID, ok
}

// Consume resolves a token and destroys it, so one link sets exactly one
// password.
func Consume(role session.Role, token string) (int, bool) {
	mu.Lock()
	defer mu.Unlock()

	found, ok := resolve(role, token)
	if !ok {
		return 0, false
	}

	delete(pending, fingerprint(token))

	return found.userID, true
}

// Clear drops every outstanding token. Tests call it to start clean.
func Clear() {
	mu.Lock()
	defer mu.Unlock()

	pending = map[string]request{}
}

// resolve looks a token up, discarding it if it has expired. Callers
// must already hold the lock.
//
// The role has to match as well, so a customer's link cannot be replayed
// against the barber form or the other way round.
func resolve(role session.Role, token string) (request, bool) {
	if token == "" {
		return request{}, false
	}

	digest := fingerprint(token)

	found, ok := pending[digest]
	if !ok {
		return request{}, false
	}

	if found.expired() {
		delete(pending, digest)

		return request{}, false
	}

	if found.role != role {
		return request{}, false
	}

	return found, true
}

// fingerprint is the digest stored in place of the token itself.
func fingerprint(token string) string {
	sum := sha256.Sum256([]byte(token))

	return hex.EncodeToString(sum[:])
}
