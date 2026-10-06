package models

type ProductOrderItem struct {
	ID int

	OrderID   int
	ProductID int
	BarberID  int

	FulfillmentStatus  string
	SellerName         string
	SellerShopName     string
	SellerShopAddress  string
	SellerPhone        string
	ProductCategory    string
	ProductDescription string
	ProductName        string
	Price              float64
	Quantity           int
	Subtotal           float64

	CustomerName  string
	CustomerPhone string
	CustomerEmail string
	OrderStatus   string
}
