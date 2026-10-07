package storage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"
)

type discoveryConnector struct{ state *discoveryDriverState }
type discoveryDriverState struct {
	queries                         int
	countError, cardError, rowError error
	badScan                         bool
}
type discoveryDriver struct{}
type discoveryConn struct{ state *discoveryDriverState }
type discoveryRows struct {
	values []driver.Value
	read   bool
	err    error
}

func (c discoveryConnector) Connect(context.Context) (driver.Conn, error) {
	return &discoveryConn{c.state}, nil
}
func (c discoveryConnector) Driver() driver.Driver       { return discoveryDriver{} }
func (discoveryDriver) Open(string) (driver.Conn, error) { return nil, errors.New("use connector") }
func (c *discoveryConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (c *discoveryConn) Close() error              { return nil }
func (c *discoveryConn) Begin() (driver.Tx, error) { return nil, errors.New("unexpected transaction") }
func (c *discoveryConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.state.queries++
	if strings.HasPrefix(query, "select count(*)") {
		if c.state.countError != nil {
			return nil, c.state.countError
		}
		return &discoveryRows{values: []driver.Value{int64(999)}}, nil
	}
	if c.state.cardError != nil {
		return nil, c.state.cardError
	}
	if strings.Contains(query, "password_hash") || strings.Contains(query, "phone_number") || strings.Contains(query, "email") {
		return nil, errors.New("private fields selected")
	}
	if len(args) != 3 || args[2].Value != int64(25) {
		return nil, errors.New("card query was not bounded")
	}
	values := []driver.Value{int64(1), "Barber", "Shop", "Address", "Lagos", "Ikeja", "", "", "Active", int64(2), float64(500), float64(4), int64(2)}
	if c.state.badScan {
		values[0] = "invalid id"
	}
	return &discoveryRows{values: values, err: c.state.rowError}, nil
}
func (r *discoveryRows) Columns() []string { return make([]string, len(r.values)) }
func (r *discoveryRows) Close() error      { return nil }
func (r *discoveryRows) Next(values []driver.Value) error {
	if r.read {
		if r.err != nil {
			return r.err
		}
		return io.EOF
	}
	r.read = true
	copy(values, r.values)
	return nil
}
func TestDiscoveryQueryCountAndFailures(t *testing.T) {
	failure := errors.New("injected discovery outage")
	for name, state := range map[string]*discoveryDriverState{"bounded summaries": {}, "count failure": {countError: failure}, "card query failure": {cardError: failure}, "row failure": {rowError: failure}, "scan failure": {badScan: true}} {
		t.Run(name, func(t *testing.T) {
			previous := db
			conn := sql.OpenDB(discoveryConnector{state})
			db = conn
			t.Cleanup(func() { db = previous; conn.Close() })
			page, err := DiscoveryPageResult(context.Background(), DiscoveryFilter{StateID: "Lagos", AreaID: "Ikeja"})
			if name == "bounded summaries" {
				if err != nil || len(page.Items) != 1 || page.Total != 999 || state.queries != 2 {
					t.Fatalf("queries=%d page=%+v err=%v", state.queries, page, err)
				}
				return
			}
			if err == nil {
				t.Fatal("failure hidden")
			}
			if !state.badScan && !errors.Is(err, failure) {
				t.Fatal("original outage lost", err)
			}
		})
	}
}
