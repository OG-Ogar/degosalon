package models

// Customer is a person who browses barbers and books haircuts.
//
// Customers register themselves; only their own account can read or
// change their bookings and chat messages.
type Customer struct {
	ID int

	FullName    string
	PhoneNumber string
	Email       string

	// PasswordHash holds the encoded PBKDF2 digest produced by the
	// security package. The plain password is never stored.
	PasswordHash string
}
