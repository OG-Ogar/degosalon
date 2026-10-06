package storage

import (
	"context"
	"errors"
	"strings"

	"barber-booking/models"
	"github.com/jackc/pgx/v5/pgconn"
)

// updateAccountRow preserves operational errors and identifies missing accounts.
func updateAccountRow(ctx context.Context, query string, args ...any) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		var conflict *pgconn.PgError
		if errors.As(err, &conflict) && conflict.Code == "23505" {
			return ErrEmailTaken
		}
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateCustomerProfile writes editable fields only. A stale profile cannot
// overwrite a password reset completed after the profile was loaded.
func UpdateCustomerProfile(ctx context.Context, customer models.Customer) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return err
	}
	if databaseEnabled() {
		return updateAccountRow(ctx, `update public.customers set full_name=$1,phone_number=$2,email=$3,profile_image=$4 where id=$5`, customer.FullName, customer.PhoneNumber, customer.Email, customer.ProfileImage, customer.ID)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, existing := range customers {
		if existing.ID != customer.ID && strings.EqualFold(existing.Email, customer.Email) {
			return ErrEmailTaken
		}
	}
	for i := range customers {
		if customers[i].ID == customer.ID {
			customer.PasswordHash = customers[i].PasswordHash
			if err := guardMediaLocked(customer.ProfileImage); err != nil {
				return err
			}
			customers[i] = customer
			return nil
		}
	}
	return ErrNotFound
}

// UpdateBarberProfile cannot change account status or the password. Suspension
// and recovery remain separate operations even when a profile edit races them.
func UpdateBarberProfile(ctx context.Context, barber models.Barber) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return err
	}
	if databaseEnabled() {
		return updateAccountRow(ctx, `update public.barbers set full_name=$1,phone_number=$2,email=$3,shop_name=$4,shop_address=$5,state=$6,local_area=$7,profile_image=$8,personal_image=$9 where id=$10 and status<>'Archived'`, barber.FullName, barber.PhoneNumber, barber.Email, barber.ShopName, barber.ShopAddress, barber.State, barber.LocalArea, barber.ProfileImage, barber.PersonalImage, barber.ID)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, existing := range barbers {
		if existing.ID != barber.ID && strings.EqualFold(existing.Email, barber.Email) {
			return ErrEmailTaken
		}
	}
	for i := range barbers {
		if barbers[i].ID == barber.ID && barbers[i].Status != models.BarberArchived {
			barber.PasswordHash = barbers[i].PasswordHash
			barber.Status = barbers[i].Status
			if err := guardMediaLocked(barber.ProfileImage, barber.PersonalImage); err != nil {
				return err
			}
			barbers[i] = barber
			return nil
		}
	}
	return ErrNotFound
}
