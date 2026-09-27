package models

type ProductOrderItem struct {
	ID int

	OrderID   int
	ProductID int
	BarberID  int

	ProductName string
	Price       float64
	Quantity    int
	Subtotal    float64
}