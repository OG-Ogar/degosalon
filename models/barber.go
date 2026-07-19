package models

type Barber struct {
	ID int

	FullName    string
	PhoneNumber string
	Email       string
	PassWord    string

	ShopName    string
	ShopAddress string

	ProfileImage string

	Status string
}