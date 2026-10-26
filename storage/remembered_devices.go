package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"barber-booking/models"
)

// SaveRememberedDeviceContext records a browser that has signed in. The token
// itself lives only in an HttpOnly cookie; the database keeps its digest so a
// leaked database row cannot be replayed as a device cookie.
func SaveRememberedDeviceContext(ctx context.Context, device models.RememberedDevice) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return err
	}
	if device.TokenDigest == "" || device.UserID <= 0 {
		return errors.New("remembered device is incomplete")
	}

	if databaseEnabled() {
		_, err := db.ExecContext(ctx,
			`delete from degosalon_internal.remembered_devices
			 where token_digest in (select token_digest from degosalon_internal.remembered_devices
			                        where expires_at<=now() order by expires_at limit 100)`)
		if err != nil {
			return err
		}
		_, err = db.ExecContext(ctx,
			`insert into degosalon_internal.remembered_devices(token_digest,role,user_id,expires_at)
			 values($1,$2,$3,$4)`,
			device.TokenDigest, device.Role, device.UserID, device.ExpiresAt)
		return err
	}

	mu.Lock()
	defer mu.Unlock()
	for digest, existing := range rememberedDevices {
		if !time.Now().Before(existing.ExpiresAt) {
			delete(rememberedDevices, digest)
		}
	}
	rememberedDevices[device.TokenDigest] = device
	return nil
}

// FindRememberedDeviceContext returns the account saved for this browser.
func FindRememberedDeviceContext(ctx context.Context, digest string) (models.RememberedDevice, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return models.RememberedDevice{}, err
	}
	if digest == "" {
		return models.RememberedDevice{}, ErrNotFound
	}

	if databaseEnabled() {
		var device models.RememberedDevice
		err := db.QueryRowContext(ctx,
			`select token_digest,role,user_id,expires_at from degosalon_internal.remembered_devices
			 where token_digest=$1 and expires_at>now()`, digest).
			Scan(&device.TokenDigest, &device.Role, &device.UserID, &device.ExpiresAt)
		if errors.Is(err, sql.ErrNoRows) {
			return models.RememberedDevice{}, ErrNotFound
		}
		if err != nil {
			return models.RememberedDevice{}, err
		}
		return device, nil
	}

	mu.RLock()
	defer mu.RUnlock()
	device, found := rememberedDevices[digest]
	if !found || !time.Now().Before(device.ExpiresAt) {
		return models.RememberedDevice{}, ErrNotFound
	}
	return device, nil
}

// DeleteRememberedDeviceContext forgets one browser, used when the visitor
// chooses another account.
func DeleteRememberedDeviceContext(ctx context.Context, digest string) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return err
	}
	if digest == "" {
		return nil
	}

	if databaseEnabled() {
		_, err := db.ExecContext(ctx, `delete from degosalon_internal.remembered_devices where token_digest=$1`, digest)
		return err
	}

	mu.Lock()
	defer mu.Unlock()
	delete(rememberedDevices, digest)
	return nil
}
