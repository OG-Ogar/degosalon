package storage

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"

	"barber-booking/models"
	"barber-booking/session"

	"github.com/jackc/pgx/v5/pgconn"
)

// ErrPasskeyExists means this authenticator credential is already registered.
var ErrPasskeyExists = errors.New("passkey is already registered")

func passkeyKey(id []byte) string { return base64.RawURLEncoding.EncodeToString(id) }

// CreatePasskeyContext stores a newly registered WebAuthn credential.
func CreatePasskeyContext(ctx context.Context, passkey models.Passkey) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return err
	}

	if passkey.CreatedAt.IsZero() {
		passkey.CreatedAt = time.Now()
	}
	key := passkeyKey(passkey.CredentialID)
	if key == "" {
		return errors.New("passkey credential id is empty")
	}

	if databaseEnabled() {
		_, err := db.ExecContext(ctx,
			`insert into degosalon_internal.passkeys(credential_id,role,user_id,email,display_name,credential,created_at)
			 values($1,$2,$3,$4,$5,$6,$7)`,
			key, passkey.Role, passkey.UserID, passkey.Email, passkey.DisplayName, string(passkey.Credential), passkey.CreatedAt)
		if err != nil {
			var conflict *pgconn.PgError
			if errors.As(err, &conflict) && conflict.Code == "23505" {
				return ErrPasskeyExists
			}
			return err
		}
		return nil
	}

	mu.Lock()
	defer mu.Unlock()
	if _, exists := passkeys[key]; exists {
		return ErrPasskeyExists
	}
	passkeys[key] = passkey
	return nil
}

// PasskeysForUserContext returns every credential registered to one account.
func PasskeysForUserContext(ctx context.Context, role session.Role, userID int) ([]models.Passkey, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if databaseEnabled() {
		rows, err := db.QueryContext(ctx,
			`select credential_id,role,user_id,email,display_name,credential,created_at,coalesce(last_used_at,'epoch'::timestamptz)
			 from degosalon_internal.passkeys where role=$1 and user_id=$2 order by created_at`,
			string(role), userID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var found []models.Passkey
		for rows.Next() {
			passkey, err := scanPasskey(rows)
			if err != nil {
				return nil, err
			}
			found = append(found, passkey)
		}
		return found, rows.Err()
	}

	mu.RLock()
	defer mu.RUnlock()
	var found []models.Passkey
	for _, passkey := range passkeys {
		if passkey.Role == string(role) && passkey.UserID == userID {
			found = append(found, passkey)
		}
	}
	return found, nil
}

// PasskeyCountContext reports whether the account has any passkey without
// loading the credential payloads. The login page only needs to know whether
// to offer fingerprint sign-in.
func PasskeyCountContext(ctx context.Context, role session.Role, userID int) (int, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return 0, err
	}

	if databaseEnabled() {
		var count int
		err := db.QueryRowContext(ctx,
			`select count(*) from degosalon_internal.passkeys where role=$1 and user_id=$2`,
			string(role), userID).Scan(&count)
		return count, err
	}

	mu.RLock()
	defer mu.RUnlock()
	count := 0
	for _, passkey := range passkeys {
		if passkey.Role == string(role) && passkey.UserID == userID {
			count++
		}
	}
	return count, nil
}

// UpdatePasskeyUseContext saves the signature counter returned by the
// authenticator after a successful sign-in.
func UpdatePasskeyUseContext(ctx context.Context, passkey models.Passkey, usedAt time.Time) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return err
	}

	key := passkeyKey(passkey.CredentialID)

	if databaseEnabled() {
		_, err := db.ExecContext(ctx,
			`update degosalon_internal.passkeys set credential=$2,last_used_at=$3 where credential_id=$1`,
			key, string(passkey.Credential), usedAt)
		return err
	}

	mu.Lock()
	defer mu.Unlock()
	if _, exists := passkeys[key]; !exists {
		return ErrNotFound
	}
	passkey.LastUsedAt = usedAt
	passkeys[key] = passkey
	return nil
}

// DeletePasskeyContext removes one credential, for example when a device is
// replaced or the account holder turns fingerprint sign-in off.
func DeletePasskeyContext(ctx context.Context, credentialID []byte) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return err
	}

	key := passkeyKey(credentialID)

	if databaseEnabled() {
		result, err := db.ExecContext(ctx, `delete from degosalon_internal.passkeys where credential_id=$1`, key)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return ErrNotFound
		}
		return nil
	}

	mu.Lock()
	defer mu.Unlock()
	if _, exists := passkeys[key]; !exists {
		return ErrNotFound
	}
	delete(passkeys, key)
	return nil
}

func scanPasskey(row interface {
	Scan(dest ...any) error
}) (models.Passkey, error) {
	var passkey models.Passkey
	var key string
	if err := row.Scan(&key, &passkey.Role, &passkey.UserID, &passkey.Email, &passkey.DisplayName, &passkey.Credential, &passkey.CreatedAt, &passkey.LastUsedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Passkey{}, ErrNotFound
		}
		return models.Passkey{}, err
	}
	id, err := base64.RawURLEncoding.DecodeString(key)
	if err != nil {
		return models.Passkey{}, err
	}
	passkey.CredentialID = id
	return passkey, nil
}
