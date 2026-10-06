package storage

import (
	"context"
	"database/sql"
	"errors"

	"barber-booking/models"
	"barber-booking/session"
)

var ErrOutstandingWork = errors.New("Resolve pending or accepted appointments and uncollected orders before archiving this account.")

// ArchiveBarber retains the account, history and media, and prevents new work.
// The account lock serializes archival with booking, checkout and verified login.
func ArchiveBarber(ctx context.Context, id int) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	if databaseEnabled() {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		var status string
		err = tx.QueryRowContext(ctx, "select status from public.barbers where id=$1 for update", id).Scan(&status)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if status == models.BarberArchived {
			return nil
		}
		var outstanding bool
		err = tx.QueryRowContext(ctx, `select exists(select 1 from public.bookings where barber_id=$1 and status in ('Pending','Accepted')) or exists(select 1 from public.product_order_items where barber_id=$1 and fulfillment_status in ('Pending','Ready'))`, id).Scan(&outstanding)
		if err != nil {
			return err
		}
		if outstanding {
			return ErrOutstandingWork
		}
		if _, err = tx.ExecContext(ctx, "update public.barbers set status='Archived' where id=$1", id); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "delete from degosalon_internal.auth_sessions where role='barber' and user_id=$1", id); err != nil {
			return err
		}
		return tx.Commit()
	}
	mu.Lock()
	if err := ctx.Err(); err != nil {
		mu.Unlock()
		return err
	}
	index := -1
	for i := range barbers {
		if barbers[i].ID == id {
			index = i
			break
		}
	}
	if index == -1 {
		mu.Unlock()
		return ErrNotFound
	}
	if barbers[index].Status != models.BarberArchived {
		for _, booking := range bookings {
			if booking.BarberID == id && (booking.Status == models.StatusPending || booking.Status == models.StatusAccepted) {
				mu.Unlock()
				return ErrOutstandingWork
			}
		}
		for _, item := range productOrderItems {
			if item.BarberID == id && (item.FulfillmentStatus == models.FulfillmentPending || item.FulfillmentStatus == models.FulfillmentReady) {
				mu.Unlock()
				return ErrOutstandingWork
			}
		}
		barbers[index].Status = models.BarberArchived
	}
	mu.Unlock()
	return session.DeleteForUserContext(ctx, session.RoleBarber, id)
}
