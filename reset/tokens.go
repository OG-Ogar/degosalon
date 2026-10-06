// Package reset issues hashed, expiring, single-use password recovery tokens.
// PostgreSQL completion commits password updates, session revocation, and token
// consumption together. Development memory retains failed completion tokens.
package reset

import (
	"barber-booking/session"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

const Lifetime = 30 * time.Minute

var ErrInvalidToken = errors.New("reset link is expired or invalid")

type request struct {
	role      session.Role
	userID    int
	expiresAt time.Time
}

func (r request) expired() bool { return !time.Now().Before(r.expiresAt) }

var (
	mu       sync.Mutex
	pending  = map[string]request{}
	database *sql.DB
)

// ConfigureDatabase is startup-only. Configured failures never fall back.
func ConfigureDatabase(conn *sql.DB) { database = conn }
func bounded(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 5*time.Second)
}
func validRole(role session.Role) bool {
	return role == session.RoleCustomer || role == session.RoleBarber
}
func validToken(token string) bool {
	if len(token) != 64 {
		return false
	}
	_, err := hex.DecodeString(token)
	return err == nil
}
func New(role session.Role, userID int) (string, error) {
	return NewContext(context.Background(), role, userID)
}
func NewContext(ctx context.Context, role session.Role, userID int) (string, error) {
	return issue(ctx, role, userID, "", nil)
}
func issue(ctx context.Context, role session.Role, userID int, recipient string, key []byte) (string, error) {
	ctx, cancel := bounded(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !validRole(role) || userID <= 0 {
		return "", ErrInvalidToken
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	token := hex.EncodeToString(buf)
	expires := time.Now().Add(Lifetime)
	var payload []byte
	if key != nil {
		var err error
		payload, err = sealDelivery(key, recipient, role, token)
		if err != nil {
			return "", err
		}
	}
	if database != nil {
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			return "", err
		}
		defer tx.Rollback()
		// The same owner lock also serializes token replacement with completion.
		if _, err := tx.ExecContext(ctx, "select pg_advisory_xact_lock(hashtextextended($1, 0))", "password-reset:"+string(role)+":"+fmt.Sprint(userID)); err != nil {
			return "", err
		}
		if _, err := tx.ExecContext(ctx, "delete from degosalon_internal.password_resets where expires_at<=now() and token_digest in (select token_digest from degosalon_internal.password_resets where expires_at<=now() order by expires_at limit 100)"); err != nil {
			return "", err
		}
		if _, err := tx.ExecContext(ctx, "delete from degosalon_internal.reset_mail_jobs where token_digest in (select token_digest from degosalon_internal.password_resets where role=$1 and user_id=$2)", role, userID); err != nil {
			return "", err
		}
		if _, err := tx.ExecContext(ctx, "insert into degosalon_internal.password_resets(role,user_id,token_digest,expires_at) values($1,$2,$3,$4) on conflict(role,user_id) do update set token_digest=excluded.token_digest,expires_at=excluded.expires_at", role, userID, fingerprint(token), expires); err != nil {
			return "", err
		}
		if payload != nil {
			if _, err := tx.ExecContext(ctx, "insert into degosalon_internal.reset_mail_jobs(token_digest,payload) values($1,$2)", fingerprint(token), payload); err != nil {
				return "", err
			}
		}
		if err := tx.Commit(); err != nil {
			return "", err
		}
		return token, nil
	}
	mu.Lock()
	defer mu.Unlock()
	for digest, existing := range pending {
		if existing.expired() || (existing.role == role && existing.userID == userID) {
			delete(pending, digest)
			delete(deliveries, digest)
		}
	}
	pending[fingerprint(token)] = request{role: role, userID: userID, expiresAt: expires}
	if payload != nil {
		deliveries[fingerprint(token)] = deliveryJob{digest: fingerprint(token), payload: payload, next: time.Now()}
	}
	return token, nil
}
func Lookup(role session.Role, token string) (int, bool) {
	id, err := LookupContext(context.Background(), role, token)
	return id, err == nil
}
func LookupContext(ctx context.Context, role session.Role, token string) (int, error) {
	ctx, cancel := bounded(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if !validRole(role) || !validToken(token) {
		return 0, ErrInvalidToken
	}
	if database != nil {
		var id int
		err := database.QueryRowContext(ctx, "select user_id from degosalon_internal.password_resets where role=$1 and token_digest=$2 and expires_at>now()", role, fingerprint(token)).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrInvalidToken
		}
		return id, err
	}
	mu.Lock()
	defer mu.Unlock()
	found, ok := resolve(role, token)
	if !ok {
		return 0, ErrInvalidToken
	}
	return found.userID, nil
}

// Consume is the token-only legacy/test API. Password recovery must use CompleteContext.
func Consume(role session.Role, token string) (int, bool) {
	if database != nil {
		ctx, cancel := bounded(context.Background())
		defer cancel()
		var id int
		err := database.QueryRowContext(ctx, "delete from degosalon_internal.password_resets where role=$1 and token_digest=$2 and expires_at>now() returning user_id", role, fingerprint(token)).Scan(&id)
		return id, err == nil
	}
	mu.Lock()
	defer mu.Unlock()
	found, ok := resolve(role, token)
	if !ok {
		return 0, false
	}
	delete(pending, fingerprint(token))
	delete(deliveries, fingerprint(token))
	return found.userID, true
}

// CompleteContext performs the durable recovery transaction. setMemory is used
// only by the local development store and must leave the password unchanged on
// error. PostgreSQL updates the role's account directly within this transaction.
func CompleteContext(ctx context.Context, role session.Role, token, hash string, setMemory func(context.Context, int, string) error) (int, error) {
	ctx, cancel := bounded(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if !validRole(role) || !validToken(token) || hash == "" {
		return 0, ErrInvalidToken
	}
	if database != nil {
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			return 0, err
		}
		defer tx.Rollback()
		var id int
		// Read owner first, then take the owner lock before locking/rechecking the
		// token, matching issuance order and preventing replacement/reset deadlocks.
		err = tx.QueryRowContext(ctx, "select user_id from degosalon_internal.password_resets where role=$1 and token_digest=$2 and expires_at>now()", role, fingerprint(token)).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrInvalidToken
		}
		if err != nil {
			return 0, err
		}
		if _, err = tx.ExecContext(ctx, "select pg_advisory_xact_lock(hashtextextended($1, 0))", "password-reset:"+string(role)+":"+fmt.Sprint(id)); err != nil {
			return 0, err
		}
		err = tx.QueryRowContext(ctx, "select user_id from degosalon_internal.password_resets where role=$1 and token_digest=$2 and expires_at>now() for update", role, fingerprint(token)).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrInvalidToken
		}
		if err != nil {
			return 0, err
		}
		table := "customers"
		if role == session.RoleBarber {
			table = "barbers"
		}
		result, err := tx.ExecContext(ctx, "update public."+table+" set password_hash=$1 where id=$2", hash, id)
		if err != nil {
			return 0, err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		if count != 1 {
			return 0, ErrInvalidToken
		}
		if _, err = tx.ExecContext(ctx, "delete from degosalon_internal.auth_sessions where role=$1 and user_id=$2", role, id); err != nil {
			return 0, err
		}
		if _, err = tx.ExecContext(ctx, "delete from degosalon_internal.password_resets where role=$1 and user_id=$2", role, id); err != nil {
			return 0, err
		}
		if err = tx.Commit(); err != nil {
			return 0, err
		}
		return id, nil
	}
	if session.DatabaseConfigured() {
		return 0, errors.New("recovery and session stores must share persistence")
	}
	mu.Lock()
	defer mu.Unlock()
	found, ok := resolve(role, token)
	if !ok {
		return 0, ErrInvalidToken
	}
	if setMemory == nil {
		return 0, errors.New("missing memory password writer")
	}
	if err := setMemory(ctx, found.userID, hash); err != nil {
		return 0, err
	}
	// Once the memory password write succeeds, finish the in-memory revocation
	// even if the client disconnects; this cannot involve a database operation.
	if err := session.DeleteForUserContext(context.Background(), role, found.userID); err != nil {
		return 0, err
	}
	delete(pending, fingerprint(token))
	delete(deliveries, fingerprint(token))
	return found.userID, nil
}

// Clear resets development test tokens only; it never deletes database records.
func Clear() {
	mu.Lock()
	defer mu.Unlock()
	pending = map[string]request{}
	deliveries = map[string]deliveryJob{}
}
func resolve(role session.Role, token string) (request, bool) {
	if !validToken(token) {
		return request{}, false
	}
	digest := fingerprint(token)
	found, ok := pending[digest]
	if !ok {
		return request{}, false
	}
	if found.expired() {
		delete(pending, digest)
		delete(deliveries, digest)
		return request{}, false
	}
	if found.role != role {
		return request{}, false
	}
	return found, true
}
func fingerprint(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
