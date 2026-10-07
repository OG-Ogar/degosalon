package storage

import (
	"barber-booking/models"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"
)

var errRowCount = errors.New("injected affected-row failure")

type rowCountDriver struct{}
type rowCountConn struct{}
type rowCountTx struct{}
type rowCountResult struct{}
type rowCountRows struct {
	values []driver.Value
	read   bool
}

func init()                                             { sql.Register("degosalon-row-count-failure", rowCountDriver{}) }
func (rowCountDriver) Open(string) (driver.Conn, error) { return rowCountConn{}, nil }
func (rowCountConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (rowCountConn) Close() error                   { return nil }
func (rowCountConn) Begin() (driver.Tx, error)      { return rowCountTx{}, nil }
func (rowCountTx) Commit() error                    { return errors.New("unexpected commit after affected-row failure") }
func (rowCountTx) Rollback() error                  { return nil }
func (rowCountResult) LastInsertId() (int64, error) { return 0, nil }
func (rowCountResult) RowsAffected() (int64, error) { return 0, errRowCount }
func (rowCountConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return rowCountResult{}, nil
}
func (rowCountConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	var values []driver.Value
	switch {
	case strings.HasPrefix(query, "select id from public.customers"):
		values = []driver.Value{int64(1)}
	case strings.HasPrefix(query, "select product_id,quantity from public.carts"):
		values = []driver.Value{int64(1), int64(1)}
	case strings.HasPrefix(query, "select p.id,p.barber_id"):
		values = []driver.Value{int64(1), int64(1), "Product", "Category", "Description", float64(99), int64(2), "", true, models.BarberActive, "Seller", "Shop", "Address", "08012345678"}
	case strings.HasPrefix(query, "insert into public.product_orders"):
		values = []driver.Value{int64(1)}
	default:
		return nil, errors.New("unexpected query after affected-row failure")
	}
	return &rowCountRows{values: values}, nil
}
func (rows *rowCountRows) Columns() []string { return make([]string, len(rows.values)) }
func (rows *rowCountRows) Close() error      { return nil }
func (rows *rowCountRows) Next(values []driver.Value) error {
	if rows.read {
		return io.EOF
	}
	rows.read = true
	copy(values, rows.values)
	return nil
}

// Exercise the public mutations through database/sql: an unavailable row count
// must remain an operational error, never not-found, lifecycle conflict or success.
func TestMutationAffectedRowFailures(t *testing.T) {
	connection, err := sql.Open("degosalon-row-count-failure", "")
	if err != nil {
		t.Fatal(err)
	}
	previous := db
	db = connection
	t.Cleanup(func() { db = previous; connection.Close() })
	ctx := context.Background()
	for name, write := range map[string]func() error{
		"product": func() error { return UpdateProductContext(ctx, models.Product{ID: 1}) },
		"service": func() error { return UpdateHairstyleContext(ctx, models.Hairstyle{ID: 1}) },
		"barber":  func() error { return UpdateBarberContext(ctx, models.Barber{ID: 1}) },
		"booking transition": func() error {
			return TransitionBookingStatusContext(ctx, 1, models.StatusAccepted, []string{models.StatusPending}, 1, 0)
		},
		"checkout stock": func() error { _, err := CheckoutCartContext(ctx, models.Customer{ID: 1}); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := write(); !errors.Is(err, errRowCount) {
				t.Fatalf("affected-row error replaced: %v", err)
			}
		})
	}
}
