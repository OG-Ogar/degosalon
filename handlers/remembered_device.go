package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"barber-booking/internal/runtimeconfig"
	"barber-booking/models"
	"barber-booking/session"
	"barber-booking/storage"
)

const (
	// rememberedDeviceLifetime is deliberately longer than a signed-in
	// session. It remembers the account name, not a login.
	rememberedDeviceLifetime = 180 * 24 * time.Hour
	rememberedDeviceCookie   = "_device"
)

func authCookieSecure() bool {
	return runtimeconfig.Production() || os.Getenv("SECURE_COOKIES") == "1"
}

func deviceCookieName(role session.Role) string {
	return string(role) + rememberedDeviceCookie
}

func deviceDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomDeviceToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// rememberDevice saves the account against this browser after a successful
// password sign-in. The cookie is HttpOnly, so page scripts cannot copy the
// identifier, and it still grants nothing without a password or passkey.
func rememberDevice(w http.ResponseWriter, r *http.Request, role session.Role, userID int) error {
	if role != session.RoleCustomer && role != session.RoleBarber {
		return errors.New("role cannot remember devices")
	}

	// Rotate rather than extend: a copied old cookie stops working as soon
	// as the real browser signs in again.
	if cookie, err := r.Cookie(deviceCookieName(role)); err == nil {
		if err := storage.DeleteRememberedDeviceContext(r.Context(), deviceDigest(cookie.Value)); err != nil {
			return err
		}
	}

	token, err := randomDeviceToken()
	if err != nil {
		return err
	}
	digest := deviceDigest(token)
	expires := time.Now().Add(rememberedDeviceLifetime)
	if err := storage.SaveRememberedDeviceContext(r.Context(), models.RememberedDevice{
		TokenDigest: digest,
		Role:        string(role),
		UserID:      userID,
		ExpiresAt:   expires,
	}); err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     deviceCookieName(role),
		Value:    token,
		Path:     "/",
		MaxAge:   int(rememberedDeviceLifetime.Seconds()),
		HttpOnly: true,
		Secure:   authCookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// rememberDeviceBestEffort keeps a storage hiccup from turning a valid login
// into an error. The visitor simply sees the email form next time.
func rememberDeviceBestEffort(w http.ResponseWriter, r *http.Request, role session.Role, userID int) {
	if err := rememberDevice(w, r, role, userID); err != nil {
		slog.Error("remember_device_failed", "request_id", w.Header().Get("X-Request-ID"), "role", role, "error", err)
	}
}

func forgetRememberedDevice(w http.ResponseWriter, r *http.Request, role session.Role) error {
	if cookie, err := r.Cookie(deviceCookieName(role)); err == nil {
		if err := storage.DeleteRememberedDeviceContext(r.Context(), deviceDigest(cookie.Value)); err != nil {
			return err
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     deviceCookieName(role),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   authCookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// rememberedAccount resolves the account saved for this browser. A deleted
// account clears the hint so the login page falls back to the email form.
func rememberedAccount(w http.ResponseWriter, r *http.Request, role session.Role) (email, name string, userID int, found bool, err error) {
	cookie, err := r.Cookie(deviceCookieName(role))
	if errors.Is(err, http.ErrNoCookie) {
		return "", "", 0, false, nil
	}
	if err != nil {
		return "", "", 0, false, err
	}

	device, err := storage.FindRememberedDeviceContext(r.Context(), deviceDigest(cookie.Value))
	if errors.Is(err, storage.ErrNotFound) {
		return "", "", 0, false, nil
	}
	if err != nil {
		return "", "", 0, false, err
	}
	if device.Role != string(role) {
		return "", "", 0, false, nil
	}

	switch role {
	case session.RoleCustomer:
		customer, err := storage.FindCustomerByIDResult(r.Context(), device.UserID)
		if errors.Is(err, storage.ErrNotFound) {
			_ = forgetRememberedDevice(w, r, role)
			return "", "", 0, false, nil
		}
		if err != nil {
			return "", "", 0, false, err
		}
		return customer.Email, customer.FullName, customer.ID, true, nil
	case session.RoleBarber:
		barber, err := storage.FindBarberByIDContext(r.Context(), device.UserID)
		if errors.Is(err, storage.ErrNotFound) {
			_ = forgetRememberedDevice(w, r, role)
			return "", "", 0, false, nil
		}
		if err != nil {
			return "", "", 0, false, err
		}
		return barber.Email, barber.FullName, barber.ID, true, nil
	default:
		return "", "", 0, false, nil
	}
}
