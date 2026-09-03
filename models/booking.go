package models

type Booking struct {
	ID          int
	BarberID    int
	HairstyleID int

	CustomerName  string
	CustomerPhone string
	CustomerEmail string

	Date string
	Time string

	Status string
}
