package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"barber-booking/models"
)

// FindCustomerByIDResult distinguishes a missing customer from a failed lookup.
func FindCustomerByIDResult(ctx context.Context, id int) (models.Customer, error) {
	if err := ctx.Err(); err != nil {
		return models.Customer{}, err
	}
	if databaseEnabled() {
		var customer models.Customer
		err := db.QueryRowContext(ctx, "select id,full_name,phone_number,email,password_hash,profile_image from public.customers where id=$1", id).Scan(&customer.ID, &customer.FullName, &customer.PhoneNumber, &customer.Email, &customer.PasswordHash, &customer.ProfileImage)
		if errors.Is(err, sql.ErrNoRows) {
			return models.Customer{}, ErrNotFound
		}
		if err != nil {
			return models.Customer{}, err
		}
		return customer, nil
	}
	mu.RLock()
	defer mu.RUnlock()
	customer, found := findCustomerByID(id)
	if !found {
		return models.Customer{}, ErrNotFound
	}
	return customer, nil
}

// FindCustomerByEmailResult preserves database errors during authentication.
func FindCustomerByEmailResult(ctx context.Context, email string) (models.Customer, error) {
	if err := ctx.Err(); err != nil {
		return models.Customer{}, err
	}
	if databaseEnabled() {
		var customer models.Customer
		err := db.QueryRowContext(ctx, "select id,full_name,phone_number,email,password_hash,profile_image from public.customers where lower(email)=lower($1)", email).Scan(&customer.ID, &customer.FullName, &customer.PhoneNumber, &customer.Email, &customer.PasswordHash, &customer.ProfileImage)
		if errors.Is(err, sql.ErrNoRows) {
			return models.Customer{}, ErrNotFound
		}
		if err != nil {
			return models.Customer{}, err
		}
		return customer, nil
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, customer := range customers {
		if strings.EqualFold(customer.Email, email) {
			return customer, nil
		}
	}
	return models.Customer{}, ErrNotFound
}

// FindBarberByIDContext is the request-aware barber lookup for session checks.
func FindBarberByIDContext(ctx context.Context, id int) (models.Barber, error) {
	if err := ctx.Err(); err != nil {
		return models.Barber{}, err
	}
	if databaseEnabled() {
		var barber models.Barber
		err := db.QueryRowContext(ctx, "select id, full_name, phone_number, email, password_hash, shop_name, shop_address, state, local_area, profile_image, personal_image, status from public.barbers where id=$1", id).Scan(&barber.ID, &barber.FullName, &barber.PhoneNumber, &barber.Email, &barber.PasswordHash, &barber.ShopName, &barber.ShopAddress, &barber.State, &barber.LocalArea, &barber.ProfileImage, &barber.PersonalImage, &barber.Status)
		if errors.Is(err, sql.ErrNoRows) {
			return models.Barber{}, ErrNotFound
		}
		if err != nil {
			return models.Barber{}, err
		}
		return barber, nil
	}
	mu.RLock()
	defer mu.RUnlock()
	barber, found := findBarberByID(id)
	if !found {
		return models.Barber{}, ErrNotFound
	}
	return barber, nil
}

// FindBarberByEmailResult preserves database errors during authentication.
func FindBarberByEmailResult(ctx context.Context, email string) (models.Barber, error) {
	if err := ctx.Err(); err != nil {
		return models.Barber{}, err
	}
	if databaseEnabled() {
		var barber models.Barber
		err := db.QueryRowContext(ctx, "select id, full_name, phone_number, email, password_hash, shop_name, shop_address, state, local_area, profile_image, personal_image, status from public.barbers where lower(email)=lower($1)", email).Scan(&barber.ID, &barber.FullName, &barber.PhoneNumber, &barber.Email, &barber.PasswordHash, &barber.ShopName, &barber.ShopAddress, &barber.State, &barber.LocalArea, &barber.ProfileImage, &barber.PersonalImage, &barber.Status)
		if errors.Is(err, sql.ErrNoRows) {
			return models.Barber{}, ErrNotFound
		}
		if err != nil {
			return models.Barber{}, err
		}
		return barber, nil
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, barber := range barbers {
		if strings.EqualFold(barber.Email, email) {
			return barber, nil
		}
	}
	return models.Barber{}, ErrNotFound
}
