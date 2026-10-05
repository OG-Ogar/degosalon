package handlers

import (
	"barber-booking/session"
	"barber-booking/storage"
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
)

func startVerifiedAccountSession(w http.ResponseWriter, r *http.Request, role session.Role, id int, hash string) error {
	_, err := session.StartVerifiedContext(r.Context(), w, role, id, hash, func(ctx context.Context) error {
		var current string
		if role == session.RoleBarber {
			account, err := storage.FindBarberByIDContext(ctx, id)
			if errors.Is(err, storage.ErrNotFound) {
				return session.ErrCredentialsChanged
			}
			if err != nil {
				return err
			}
			if !account.IsActive() {
				return session.ErrCredentialsChanged
			}
			current = account.PasswordHash
		} else {
			account, err := storage.FindCustomerByIDResult(ctx, id)
			if errors.Is(err, storage.ErrNotFound) {
				return session.ErrCredentialsChanged
			}
			if err != nil {
				return err
			}
			current = account.PasswordHash
		}
		if subtle.ConstantTimeCompare([]byte(current), []byte(hash)) != 1 {
			return session.ErrCredentialsChanged
		}
		return nil
	})
	return err
}
