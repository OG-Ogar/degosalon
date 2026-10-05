package storage

import (
	"context"
	"strings"

	"barber-booking/models"
)

// ListCustomers returns every registered customer.
func ListCustomers() []models.Customer {
	if databaseEnabled() {
		rows, err := db.Query("select id,full_name,phone_number,email,password_hash,profile_image from public.customers order by id")
		if err == nil {
			defer rows.Close()
			var out []models.Customer
			for rows.Next() {
				var v models.Customer
				if rows.Scan(&v.ID, &v.FullName, &v.PhoneNumber, &v.Email, &v.PasswordHash, &v.ProfileImage) == nil {
					out = append(out, v)
				}
			}
			return out
		}
	}
	mu.RLock()
	defer mu.RUnlock()

	return append([]models.Customer(nil), customers...)
}

// FindCustomerByID returns a copy of the customer with the given ID.
func FindCustomerByID(id int) (models.Customer, bool) {
	if databaseEnabled() {
		var v models.Customer
		err := db.QueryRow("select id,full_name,phone_number,email,password_hash,profile_image from public.customers where id=$1", id).Scan(&v.ID, &v.FullName, &v.PhoneNumber, &v.Email, &v.PasswordHash, &v.ProfileImage)
		return v, err == nil
	}
	mu.RLock()
	defer mu.RUnlock()

	return findCustomerByID(id)
}

// findCustomerByID is the unlocked form used by callers holding the lock.
func findCustomerByID(id int) (models.Customer, bool) {
	for _, customer := range customers {
		if customer.ID == id {
			return customer, true
		}
	}

	return models.Customer{}, false
}

// FindCustomerByEmail matches case-insensitively, the way logins do.
func FindCustomerByEmail(email string) (models.Customer, bool) {
	if databaseEnabled() {
		var v models.Customer
		err := db.QueryRow("select id,full_name,phone_number,email,password_hash,profile_image from public.customers where lower(email)=lower($1)", email).Scan(&v.ID, &v.FullName, &v.PhoneNumber, &v.Email, &v.PasswordHash, &v.ProfileImage)
		return v, err == nil
	}
	mu.RLock()
	defer mu.RUnlock()

	for _, customer := range customers {
		if strings.EqualFold(customer.Email, email) {
			return customer, true
		}
	}

	return models.Customer{}, false
}

// CreateCustomer assigns an ID and stores the customer, rejecting an
// email that is already registered.
func CreateCustomer(customer models.Customer) (models.Customer, error) {
	if databaseEnabled() {
		err := db.QueryRow("insert into public.customers(full_name,phone_number,email,password_hash,profile_image) values($1,$2,$3,$4,$5) returning id", customer.FullName, customer.PhoneNumber, customer.Email, customer.PasswordHash, customer.ProfileImage).Scan(&customer.ID)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				return models.Customer{}, ErrEmailTaken
			}
			return models.Customer{}, err
		}
		return customer, nil
	}
	mu.Lock()
	defer mu.Unlock()

	for _, existing := range customers {
		if strings.EqualFold(existing.Email, customer.Email) {
			return models.Customer{}, ErrEmailTaken
		}
	}

	customer.ID = nextID(&lastCustomerID)

	customers = append(customers, customer)

	return customer, nil
}

// SetCustomerPassword replaces one customer's stored password hash.
//
// It exists so a password reset does not have to read the customer out,
// change one field and write the whole record back: that pair of calls
// would let a profile edit in between be silently overwritten.
func SetCustomerPassword(id int, hash string) error {
	return SetCustomerPasswordContext(context.Background(), id, hash)
}

func SetCustomerPasswordContext(ctx context.Context, id int, hash string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if databaseEnabled() {
		return updateAccountRow(ctx, "update public.customers set password_hash=$1 where id=$2", hash, id)
	}
	mu.Lock()
	defer mu.Unlock()

	for i := range customers {
		if customers[i].ID == id {
			customers[i].PasswordHash = hash

			return nil
		}
	}

	return ErrNotFound
}

// UpdateCustomer overwrites the stored customer that shares its ID,
// keeping the existing password hash when the incoming copy omits it.
func UpdateCustomer(customer models.Customer) error {
	if databaseEnabled() {
		return updateAccountRow(context.Background(), `update public.customers set full_name=$1,phone_number=$2,email=$3,profile_image=$4,password_hash=coalesce(nullif($5,''),password_hash) where id=$6`, customer.FullName, customer.PhoneNumber, customer.Email, customer.ProfileImage, customer.PasswordHash, customer.ID)
	}
	mu.Lock()
	defer mu.Unlock()

	for _, existing := range customers {
		if existing.ID != customer.ID &&
			strings.EqualFold(existing.Email, customer.Email) {

			return ErrEmailTaken
		}
	}

	for i := range customers {
		if customers[i].ID != customer.ID {
			continue
		}

		if customer.PasswordHash == "" {
			customer.PasswordHash = customers[i].PasswordHash
		}

		customers[i] = customer

		return nil
	}

	return ErrNotFound
}
