package models

type Hairstyle struct {
	ID          int
	BarberID    int
	Name        string
	Description string
	Price       float64
	Image       string
	Status      string
}