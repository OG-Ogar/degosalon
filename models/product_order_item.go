package models

type ProductOrderItem struct {
	ID int

	OrderID   int
	ProductID int
	BarberID  int

	FulfillmentStatus string
	ProductName       string
	Price             float64
	Quantity          int
	Subtotal          float64

	CustomerName  string
	CustomerPhone string
	CustomerEmail string
	OrderStatus   string
}
