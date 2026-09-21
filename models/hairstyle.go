package models

type Hairstyle struct {
	ID       int
	BarberID int

	Name        string
	Category    string
	Description string

	Price    float64
	Duration int

	Image string
	Video string

	IsAvailable bool
}
