package storage

import (
	"strings"

	"barber-booking/models"
)

// ListCustomers returns every registered customer.
func ListCustomers() []models.Customer {
	mu.RLock()
	defer mu.RUnlock()

	return append([]models.Customer(nil), customers...)
}

// FindCustomerByID returns a copy of the customer with the given ID.
func FindCustomerByID(id int) (models.Customer, bool) {
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

// UpdateCustomer overwrites the stored customer that shares its ID,
// keeping the existing password hash when the incoming copy omits it.
func UpdateCustomer(customer models.Customer) error {
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
