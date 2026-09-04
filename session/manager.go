// Package session keeps signed-in users in memory and owns the cookie
// contract used by the admin, barber and customer logins.
//
// Every role gets its own cookie name, so one browser can be signed in
// as a barber and a customer at the same time. Sessions carry a CSRF
// token that state-changing requests must echo back.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

// Role identifies which part of the site a session belongs to.
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleBarber   Role = "barber"
	RoleCustomer Role = "customer"
)

// Lifetime is how long a session stays valid after it is created.
const Lifetime = 12 * time.Hour

// Session is one signed-in user.
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
)

// CookieName is the cookie that carries the session ID for a role.
func CookieName(role Role) string {
	return string(role) + "_session"
}

// secureCookies reports whether cookies should be marked Secure. It is
// off by default because the development server speaks plain HTTP; set
// SECURE_COOKIES=1 when serving the site over HTTPS.
func secureCookies() bool {
	return os.Getenv("SECURE_COOKIES") == "1"
}

// randomID returns a 32 character cryptographically random hex string.
func randomID() (string, error) {
	buf := make([]byte, 16)

	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}

	return hex.EncodeToString(buf), nil
}

// New creates and stores a session for the given role and user ID.
func New(role Role, userID int) (Session, error) {
	id, err := randomID()
	if err != nil {
		return Session{}, err
	}

	token, err := randomID()
	if err != nil {
		return Session{}, err
	}

	s := Session{
		ID:        id,
		Role:      role,
		UserID:    userID,
		CSRFToken: token,
		ExpiresAt: time.Now().Add(Lifetime),
	}

	mu.Lock()
	defer mu.Unlock()

	// Opportunistically drop anything that has already expired so the
	// map does not grow without bound.
	for key, existing := range sessions {
		if time.Now().After(existing.ExpiresAt) {
			delete(sessions, key)
		}
	}

	sessions[id] = s

	return s, nil
}

// Get returns the session with the given ID. Expired sessions are
// removed and reported as missing.
func Get(id string) (Session, bool) {
	mu.Lock()
	defer mu.Unlock()

	s, found := sessions[id]
	if !found {
		return Session{}, false
	}

	if time.Now().After(s.ExpiresAt) {
		delete(sessions, id)
		return Session{}, false
	}

	return s, true
}

// Delete removes a single session, used on logout.
func Delete(id string) {
	mu.Lock()
	defer mu.Unlock()

	delete(sessions, id)
}

// DeleteForUser removes every session belonging to one account. The
// admin calls this after suspending or deleting a barber so that any
// browser already signed in as that barber is signed straight out.
func DeleteForUser(role Role, userID int) {
	mu.Lock()
	defer mu.Unlock()

	for id, s := range sessions {
		if s.Role == role && s.UserID == userID {
			delete(sessions, id)
		}
	}
}

// Start creates a session and writes its cookie to the response.
func Start(w http.ResponseWriter, role Role, userID int) (Session, error) {
	s, err := New(role, userID)
	if err != nil {
		return Session{}, err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName(role),
		Value:    s.ID,
		Path:     "/",
		MaxAge:   int(Lifetime.Seconds()),
		HttpOnly: true,
		Secure:   secureCookies(),
		SameSite: http.SameSiteLaxMode,
	})

	return s, nil
}

// Clear deletes the session named by the role's cookie and expires the
// cookie in the browser.
func Clear(w http.ResponseWriter, r *http.Request, role Role) {
	if cookie, err := r.Cookie(CookieName(role)); err == nil {
		Delete(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName(role),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookies(),
		SameSite: http.SameSiteLaxMode,
	})
}

// FromRequest reads the cookie for a role and returns its session.
func FromRequest(r *http.Request, role Role) (Session, bool) {
	cookie, err := r.Cookie(CookieName(role))
	if err != nil {
		return Session{}, false
	}

	s, found := Get(cookie.Value)
	if !found || s.Role != role {
		return Session{}, false
	}

	return s, true
}
