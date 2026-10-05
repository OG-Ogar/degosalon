package storage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"testing"

	"barber-booking/models"
)

func TestAccountLookupsInMemory(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	customer, err := CreateCustomer(models.Customer{Email: "customer@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	barber, err := CreateBarber(models.Barber{Email: "barber@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	gotCustomer, err := FindCustomerByIDResult(ctx, customer.ID)
	if err != nil || gotCustomer.ID != customer.ID {
		t.Fatalf("customer ID lookup: %+v, %v", gotCustomer, err)
	}
	gotCustomer, err = FindCustomerByEmailResult(ctx, "CUSTOMER@example.com")
	if err != nil || gotCustomer.ID != customer.ID {
		t.Fatalf("customer email lookup: %+v, %v", gotCustomer, err)
	}
	gotBarber, err := FindBarberByIDContext(ctx, barber.ID)
	if err != nil || gotBarber.ID != barber.ID {
		t.Fatalf("barber ID lookup: %+v, %v", gotBarber, err)
	}
	gotBarber, err = FindBarberByEmailResult(ctx, "BARBER@example.com")
	if err != nil || gotBarber.ID != barber.ID {
		t.Fatalf("barber email lookup: %+v, %v", gotBarber, err)
	}
	for name, lookup := range accountLookupCalls() {
		t.Run(name, func(t *testing.T) {
			if err := lookup(ctx); !errors.Is(err, ErrNotFound) {
				t.Fatalf("missing account: %v", err)
			}
			canceled, cancel := context.WithCancel(ctx)
			cancel()
			if err := lookup(canceled); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled lookup: %v", err)
			}
		})
	}
}

func accountLookupCalls() map[string]func(context.Context) error {
	return map[string]func(context.Context) error{
		"customer ID": func(ctx context.Context) error { _, err := FindCustomerByIDResult(ctx, 99); return err },
		"customer email": func(ctx context.Context) error {
			_, err := FindCustomerByEmailResult(ctx, "missing@example.com")
			return err
		},
		"barber ID": func(ctx context.Context) error { _, err := FindBarberByIDContext(ctx, 99); return err },
		"barber email": func(ctx context.Context) error {
			_, err := FindBarberByEmailResult(ctx, "missing@example.com")
			return err
		},
	}
}

// Controlled SQL responses exercise database/sql's query, Scan, and no-row paths.
// This does not require a live PostgreSQL server.
type accountConnector struct {
	queryErr   error
	invalidRow bool
}

func (c accountConnector) Connect(context.Context) (driver.Conn, error) { return accountConn{c}, nil }
func (c accountConnector) Driver() driver.Driver                        { return accountDriver{c} }

type accountDriver struct{ connector accountConnector }

func (d accountDriver) Open(string) (driver.Conn, error) { return accountConn{d.connector}, nil }

type accountConn struct{ accountConnector }

func (c accountConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (c accountConn) Close() error              { return nil }
func (c accountConn) Begin() (driver.Tx, error) { return nil, errors.New("unexpected transaction") }
func (c accountConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	if c.queryErr != nil {
		return nil, c.queryErr
	}
	return &accountRows{invalid: c.invalidRow}, nil
}

type accountRows struct{ invalid, done bool }

func (r *accountRows) Columns() []string { return []string{"invalid"} }
func (r *accountRows) Close() error      { return nil }
func (r *accountRows) Next(dest []driver.Value) error {
	if !r.invalid || r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = "invalid account row"
	return nil
}

func TestAccountLookupsDistinguishDatabaseFailuresFromMissingRecords(t *testing.T) {
	failure := errors.New("database unavailable")
	for _, tc := range []struct {
		name      string
		connector accountConnector
		want      error
	}{
		{"query failure", accountConnector{queryErr: failure}, failure},
		{"missing row", accountConnector{}, ErrNotFound},
		{"invalid row", accountConnector{invalidRow: true}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previous := db
			conn := sql.OpenDB(tc.connector)
			db = conn
			t.Cleanup(func() { db = previous; conn.Close() })
			for name, lookup := range accountLookupCalls() {
				t.Run(name, func(t *testing.T) {
					err := lookup(context.Background())
					if tc.want != nil {
						if !errors.Is(err, tc.want) {
							t.Fatalf("got %v, want %v", err, tc.want)
						}
					} else if err == nil || errors.Is(err, ErrNotFound) {
						t.Fatalf("scan failure treated as missing: %v", err)
					}
				})
			}
		})
	}
}
