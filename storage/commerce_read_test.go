package storage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"barber-booking/models"
)

func TestCommerceReadsPropagateQueryAndScanFailures(t *testing.T) {
	failure := errors.New("commerce database unavailable")
	for _, tc := range []struct {
		name      string
		connector accountConnector
		want      error
	}{
		{"query", accountConnector{queryErr: failure}, failure},
		{"scan", accountConnector{invalidRow: true}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previous := db
			conn := sql.OpenDB(tc.connector)
			db = conn
			t.Cleanup(func() { db = previous; conn.Close() })
			for name, read := range map[string]func() error{
				"catalogue":     func() error { _, err := ListAvailableProducts(context.Background()); return err },
				"inventory":     func() error { _, err := ListProductsByBarber(context.Background(), 1); return err },
				"cart":          func() error { _, err := ListCartItems(context.Background(), 1); return err },
				"cart count":    func() error { _, err := CartItemCount(context.Background(), 1); return err },
				"order":         func() error { _, err := FindProductOrderByIDResult(context.Background(), 1); return err },
				"order items":   func() error { _, err := ListProductOrderItems(context.Background(), 1); return err },
				"seller sales":  func() error { _, err := ListProductOrderItemsByBarber(context.Background(), 1); return err },
				"owned product": func() error { _, err := FindProductOwnedBy(context.Background(), 1, 1); return err },
			} {
				t.Run(name, func(t *testing.T) {
					err := read()
					if err == nil || errors.Is(err, ErrNotFound) {
						t.Fatalf("failure hidden: %v", err)
					}
					if tc.want != nil && !errors.Is(err, tc.want) {
						t.Fatalf("got %v want %v", err, tc.want)
					}
				})
			}
		})
	}
}

// A row-iteration failure after a valid row must discard the partial result.
type interruptedConnector struct{ failure error }

func (c interruptedConnector) Connect(context.Context) (driver.Conn, error) {
	return interruptedConn{c.failure}, nil
}
func (c interruptedConnector) Driver() driver.Driver { return accountDriver{} }

type interruptedConn struct{ failure error }

func (c interruptedConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (c interruptedConn) Close() error              { return nil }
func (c interruptedConn) Begin() (driver.Tx, error) { return nil, errors.New("unexpected transaction") }
func (c interruptedConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &interruptedRows{failure: c.failure}, nil
}

type interruptedRows struct {
	failure error
	read    bool
}

func (r *interruptedRows) Columns() []string { return []string{"value"} }
func (r *interruptedRows) Close() error      { return nil }
func (r *interruptedRows) Next(dest []driver.Value) error {
	if r.read {
		return r.failure
	}
	r.read = true
	dest[0] = int64(1)
	return nil
}

func TestQueryRecordsDiscardsPartialResults(t *testing.T) {
	failure := errors.New("interrupted rows")
	previous := db
	conn := sql.OpenDB(interruptedConnector{failure})
	db = conn
	t.Cleanup(func() { db = previous; conn.Close() })
	records, err := queryRecords(context.Background(), "test", func(rows *sql.Rows) (int, error) { var value int; err := rows.Scan(&value); return value, err })
	if !errors.Is(err, failure) || records != nil {
		t.Fatalf("partial results escaped: %v, %v", records, err)
	}
}

func TestSellerSalesRemainScopedAndIncludeCustomerDetails(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	order, err := CreateProductOrder(models.ProductOrder{CustomerID: 1, CustomerName: "Buyer", Status: models.OrderPending})
	if err != nil {
		t.Fatal(err)
	}
	for _, email := range []string{"sales_one@example.com", "sales_two@example.com"} {
		seller := newBarber(t, email)
		product := postgresProduct(t, seller.ID, 1)
		if _, err := CreateProductOrderItem(models.ProductOrderItem{OrderID: order.ID, ProductID: product.ID, BarberID: seller.ID, ProductName: "Item", Quantity: 1}); err != nil {
			t.Fatal(err)
		}
	}
	items, err := ListProductOrderItemsByBarber(context.Background(), 1)
	if err != nil || len(items) != 1 || items[0].BarberID != 1 || items[0].CustomerName != "Buyer" || items[0].OrderStatus != models.OrderPending {
		t.Fatalf("sales: %+v, %v", items, err)
	}
}
