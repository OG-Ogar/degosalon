package models

type Product struct {
	ID       int
	BarberID int

	Name        string
	Category    string
	Description string

	Price    float64
	Quantity int

	Image       string
	IsAvailable bool
}