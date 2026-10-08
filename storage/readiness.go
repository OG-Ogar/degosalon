package storage

import (
	"barber-booking/internal/runtimeconfig"
	"context"
	"errors"
)

// CheckReadiness checks connectivity without loading application records.
// Schema compatibility and required configuration are validated at startup.
func CheckReadiness(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if db == nil {
		if runtimeconfig.Production() {
			return errors.New("persistent storage unavailable")
		}
		return nil
	}
	return db.PingContext(ctx)
}
