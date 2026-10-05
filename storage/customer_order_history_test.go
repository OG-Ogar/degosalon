package storage

import (
	"context"
	"database/sql"
	"testing"

	"barber-booking/models"
)

func TestCustomerOrderHistoryIsOwnedAndNewestFirst(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	for _, owner := range []int{1, 2, 1} {
		if _, err := CreateProductOrder(models.ProductOrder{CustomerID: owner}); err != nil {
			t.Fatal(err)
		}
	}
	orders, err := ListProductOrdersByCustomer(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(orders) != 2 || orders[0].ID != 3 || orders[1].ID != 1 {
		t.Fatalf("unexpected history: %+v", orders)
	}
	empty, err := ListProductOrdersByCustomer(context.Background(), 99)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty history: %+v, %v", empty, err)
	}
}

func TestCustomerOrderHistoryDoesNotFallBackOnDatabaseFailure(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	productOrders = []models.ProductOrder{{ID: 1, CustomerID: 1}}
	conn, err := sql.Open("pgx", "postgres://localhost/unused")
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	previous := db
	db = conn
	t.Cleanup(func() { db = previous })
	orders, err := ListProductOrdersByCustomer(context.Background(), 1)
	if err == nil || orders != nil {
		t.Fatalf("database failure returned history: %+v, %v", orders, err)
	}
}
