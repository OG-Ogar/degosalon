// Package session owns role-isolated authentication cookies and CSRF tokens.
// Production sessions persist in PostgreSQL; local development can use memory.
package session

import (
	"barber-booking/internal/runtimeconfig"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleBarber   Role = "barber"
	RoleCustomer Role = "customer"
)
const Lifetime = 12 * time.Hour

var ErrNotFound = errors.New("session not found")
var ErrCredentialsChanged = errors.New("account credentials changed")

type Session struct {
	ID        string
	Role      Role
	UserID    int
	CSRFToken string
	ExpiresAt time.Time
}

var (
	mu       sync.Mutex
	sessions = map[string]Session{}
	database *sql.DB
)

// ConfigureDatabase is called at startup, before serving requests. Passing nil
// selects the development store. A configured SQL failure never falls back.
func ConfigureDatabase(conn *sql.DB) { database = conn }
func DatabaseConfigured() bool       { return database != nil }
func CookieName(role Role) string    { return string(role) + "_session" }
func secureCookies() bool            { return runtimeconfig.Production() || os.Getenv("SECURE_COOKIES") == "1" }
func randomID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
func digest(id string) string { sum := sha256.Sum256([]byte(id)); return hex.EncodeToString(sum[:]) }
func validIdentity(role Role, userID int) bool {
	return (role == RoleAdmin && userID == 0) || ((role == RoleBarber || role == RoleCustomer) && userID > 0)
}
func bounded(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 5*time.Second)
}

func New(role Role, userID int) (Session, error) {
	return NewContext(context.Background(), role, userID)
}
func NewContext(ctx context.Context, role Role, userID int) (Session, error) {
	return newSession(ctx, role, userID, nil, nil)
}

// NewVerifiedContext rechecks credentials and barber eligibility while holding
// the account row lock until the session is committed. The memory checker runs
// under the session lock, so account recovery cannot miss a session it creates.
func NewVerifiedContext(ctx context.Context, role Role, userID int, verifiedHash string, checkMemory func(context.Context) error) (Session, error) {
	if role != RoleCustomer && role != RoleBarber {
		return Session{}, ErrCredentialsChanged
	}
	return newSession(ctx, role, userID, &verifiedHash, checkMemory)
}

func newSession(ctx context.Context, role Role, userID int, verifiedHash *string, checkMemory func(context.Context) error) (Session, error) {
	ctx, cancel := bounded(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	if !validIdentity(role, userID) {
		return Session{}, errors.New("invalid session identity")
	}
	id, err := randomID()
	if err != nil {
		return Session{}, err
	}
	token, err := randomID()
	if err != nil {
		return Session{}, err
	}
	s := Session{ID: id, Role: role, UserID: userID, CSRFToken: token, ExpiresAt: time.Now().Add(Lifetime)}
	if database != nil {
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			return Session{}, err
		}
		defer tx.Rollback()
		if verifiedHash != nil {
			var current string
			active := true
			if role == RoleBarber {
				err = tx.QueryRowContext(ctx, "select password_hash,status='Active' from public.barbers where id=$1 for share", userID).Scan(&current, &active)
			} else {
				err = tx.QueryRowContext(ctx, "select password_hash from public.customers where id=$1 for share", userID).Scan(&current)
			}
			if errors.Is(err, sql.ErrNoRows) {
				return Session{}, ErrCredentialsChanged
			}
			if err != nil {
				return Session{}, err
			}
			if !active || subtle.ConstantTimeCompare([]byte(current), []byte(*verifiedHash)) != 1 {
				return Session{}, ErrCredentialsChanged
			}
		}
		if _, err := tx.ExecContext(ctx, "delete from degosalon_internal.auth_sessions where id_digest in (select id_digest from degosalon_internal.auth_sessions where expires_at<=now() order by expires_at limit 100)"); err != nil {
			return Session{}, err
		}
		if _, err := tx.ExecContext(ctx, "insert into degosalon_internal.auth_sessions(id_digest,role,user_id,csrf_token,expires_at) values($1,$2,$3,$4,$5)", digest(id), role, userID, token, s.ExpiresAt); err != nil {
			return Session{}, err
		}
		if err := tx.Commit(); err != nil {
			return Session{}, err
		}
		return s, nil
	}
	mu.Lock()
	defer mu.Unlock()
	if verifiedHash != nil {
		if checkMemory == nil {
			return Session{}, errors.New("missing credential checker")
		}
		if err := checkMemory(ctx); err != nil {
			return Session{}, err
		}
	}
	for key, existing := range sessions {
		if !time.Now().Before(existing.ExpiresAt) {
			delete(sessions, key)
		}
	}
	sessions[digest(id)] = s
	return s, nil
}
func Get(id string) (Session, bool) {
	s, err := GetContext(context.Background(), id)
	return s, err == nil
}
func GetContext(ctx context.Context, id string) (Session, error) {
	ctx, cancel := bounded(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	if len(id) != 64 {
		return Session{}, ErrNotFound
	}
	if _, err := hex.DecodeString(id); err != nil {
		return Session{}, ErrNotFound
	}
	if database != nil {
		var s Session
		err := database.QueryRowContext(ctx, "select role,user_id,csrf_token,expires_at from degosalon_internal.auth_sessions where id_digest=$1 and expires_at>now()", digest(id)).Scan(&s.Role, &s.UserID, &s.CSRFToken, &s.ExpiresAt)
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrNotFound
		}
		if err != nil {
			return Session{}, err
		}
		s.ID = id
		return s, nil
	}
	mu.Lock()
	defer mu.Unlock()
	s, ok := sessions[digest(id)]
	if !ok {
		return Session{}, ErrNotFound
	}
	if !time.Now().Before(s.ExpiresAt) {
		delete(sessions, digest(id))
		return Session{}, ErrNotFound
	}
	return s, nil
}
func Delete(id string) { _ = DeleteContext(context.Background(), id) }
func DeleteContext(ctx context.Context, id string) error {
	ctx, cancel := bounded(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	if database != nil {
		_, err := database.ExecContext(ctx, "delete from degosalon_internal.auth_sessions where id_digest=$1", digest(id))
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	delete(sessions, digest(id))
	return nil
}
func DeleteForUser(role Role, userID int) {
	_ = DeleteForUserContext(context.Background(), role, userID)
}
func DeleteForUserContext(ctx context.Context, role Role, userID int) error {
	ctx, cancel := bounded(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	if database != nil {
		_, err := database.ExecContext(ctx, "delete from degosalon_internal.auth_sessions where role=$1 and user_id=$2", role, userID)
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	for id, s := range sessions {
		if s.Role == role && s.UserID == userID {
			delete(sessions, id)
		}
	}
	return nil
}
func Start(w http.ResponseWriter, role Role, userID int) (Session, error) {
	return StartContext(context.Background(), w, role, userID)
}
func StartContext(ctx context.Context, w http.ResponseWriter, role Role, userID int) (Session, error) {
	s, err := NewContext(ctx, role, userID)
	if err != nil {
		return Session{}, err
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName(role), Value: s.ID, Path: "/", MaxAge: int(Lifetime.Seconds()), HttpOnly: true, Secure: secureCookies(), SameSite: http.SameSiteLaxMode})
	return s, nil
}

// StartVerifiedContext writes a cookie only after the checked session commits.
func StartVerifiedContext(ctx context.Context, w http.ResponseWriter, role Role, userID int, verifiedHash string, checkMemory func(context.Context) error) (Session, error) {
	s, err := NewVerifiedContext(ctx, role, userID, verifiedHash, checkMemory)
	if err != nil {
		return Session{}, err
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName(role), Value: s.ID, Path: "/", MaxAge: int(Lifetime.Seconds()), HttpOnly: true, Secure: secureCookies(), SameSite: http.SameSiteLaxMode})
	return s, nil
}

func Clear(w http.ResponseWriter, r *http.Request, role Role) { _ = ClearContext(w, r, role) }
func ClearContext(w http.ResponseWriter, r *http.Request, role Role) error {
	if cookie, err := r.Cookie(CookieName(role)); err == nil {
		if err := DeleteContext(r.Context(), cookie.Value); err != nil {
			return err
		}
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName(role), Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: secureCookies(), SameSite: http.SameSiteLaxMode})
	return nil
}
func FromRequest(r *http.Request, role Role) (Session, bool) {
	s, err := FromRequestContext(r, role)
	return s, err == nil
}
func FromRequestContext(r *http.Request, role Role) (Session, error) {
	cookie, err := r.Cookie(CookieName(role))
	if errors.Is(err, http.ErrNoCookie) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	s, err := GetContext(r.Context(), cookie.Value)
	if err != nil {
		return Session{}, err
	}
	if s.Role != role {
		return Session{}, ErrNotFound
	}
	return s, nil
}

// PruneExpired prunes expired sessions from the active store (database or in-memory map).
// It returns the number of deleted records or any error encountered.
func PruneExpired(ctx context.Context) (int64, error) {
	ctx, cancel := bounded(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if database != nil {
		res, err := database.ExecContext(ctx, "delete from degosalon_internal.auth_sessions where expires_at<=now()")
		if err != nil {
			return 0, err
		}
		return res.RowsAffected()
	}
	mu.Lock()
	defer mu.Unlock()
	var count int64
	now := time.Now()
	for key, s := range sessions {
		if !now.Before(s.ExpiresAt) {
			delete(sessions, key)
			count++
		}
	}
	return count, nil
}

// RunCleanupWorker periodically prunes expired sessions in the background
// until the supplied context is canceled.
func RunCleanupWorker(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = PruneExpired(ctx)
		}
	}
}
