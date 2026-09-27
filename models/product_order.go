package models

import "time"

const (
	OrderPending = "Pending"
	OrderPaid    = "Paid"
	OrderCancelled = "Cancelled"
	OrderCompleted = "Completed"
)

type ProductOrder struct {
	ID int

	CustomerID    int
	CustomerName  string
	CustomerPhone string
	CustomerEmail string

	TotalAmount float64

	Status string

	CreatedAt time.Time
}