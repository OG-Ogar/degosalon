package models

// Barber account statuses. A suspended barber cannot log in and is
// hidden from customers.
const (
	BarberActive    = "Active"
	BarberSuspended = "Suspended"
)

// Barber is a shop owner who publishes hairstyles and takes bookings.
type Barber struct {
	ID int

	FullName    string
	PhoneNumber string
	Email       string

	// PasswordHash holds the encoded PBKDF2 digest produced by the
	// security package. The plain password is never stored.
	PasswordHash string

	ShopName    string
	ShopAddress string
	State       string
	LocalArea   string

	// The two photos are separate on purpose. ProfileImage is the shop
	// itself, shown wide at the top of the page; PersonalImage is the
	// barber, shown beside their name so a customer knows who will be
	// cutting their hair. Either may be empty.
	ProfileImage  string
	PersonalImage string

	Status string
}

// IsActive reports whether the barber may log in and receive bookings.
func (b Barber) IsActive() bool {
	return b.Status == BarberActive
}
