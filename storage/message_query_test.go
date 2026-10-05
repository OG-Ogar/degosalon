package storage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"barber-booking/models"
)

func TestMessageReadsPreserveDatabaseFailures(t *testing.T) {
	failure := errors.New("message database unavailable")
	previous := db
	conn := sql.OpenDB(accountConnector{queryErr: failure})
	db = conn
	t.Cleanup(func() { db = previous; conn.Close() })
	for name, read := range map[string]func() error{
		"customer summaries": func() error { _, e := ConversationsForCustomer(context.Background(), 1); return e },
		"barber summaries":   func() error { _, e := ConversationsForBarber(context.Background(), 1); return e },
		"thread":             func() error { _, e := ListConversation(context.Background(), 1, 1); return e },
		"send": func() error {
			_, e := AddMessage(context.Background(), models.Message{BarberID: 1, CustomerID: 1, Text: "hello"})
			return e
		},
	} {
		t.Run(name, func(t *testing.T) {
			if e := read(); !errors.Is(e, failure) {
				t.Fatalf("hidden failure: %v", e)
			}
		})
	}
}

func TestMessageThreadsAndSummariesAreScoped(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	for _, email := range []string{"barber1@example.com", "barber2@example.com"} {
		if _, e := CreateBarber(models.Barber{Email: email}); e != nil {
			t.Fatal(e)
		}
	}
	for _, email := range []string{"customer1@example.com", "customer2@example.com"} {
		if _, e := CreateCustomer(models.Customer{Email: email}); e != nil {
			t.Fatal(e)
		}
	}
	for i, pair := range [][2]int{{1, 1}, {2, 1}, {1, 2}, {1, 1}} {
		if _, e := AddMessage(context.Background(), models.Message{BarberID: pair[0], CustomerID: pair[1], Text: []string{"first", "other barber", "other customer", "latest"}[i], SentAt: time.Unix(int64(i+1), 0)}); e != nil {
			t.Fatal(e)
		}
	}
	thread, e := ListConversation(context.Background(), 1, 1)
	if e != nil || len(thread) != 2 || thread[0].Text != "first" || thread[1].Text != "latest" {
		t.Fatalf("thread: %+v %v", thread, e)
	}
	for _, role := range []string{"customer", "barber"} {
		var list []Conversation
		if role == "customer" {
			list, e = ConversationsForCustomer(context.Background(), 1)
		} else {
			list, e = ConversationsForBarber(context.Background(), 1)
		}
		if e != nil || len(list) != 2 || list[0].LastText != "latest" || list[0].Count != 2 {
			t.Fatalf("%s summary: %+v %v", role, list, e)
		}
		for _, v := range list {
			if role == "customer" && v.CustomerID != 1 || role == "barber" && v.BarberID != 1 {
				t.Fatalf("unscoped summary: %+v", v)
			}
		}
	}
}

type scopedConnector struct {
	query *string
	args  *[]driver.NamedValue
}

func (c scopedConnector) Connect(context.Context) (driver.Conn, error) { return scopedConn{c}, nil }
func (c scopedConnector) Driver() driver.Driver                        { return accountDriver{} }

type scopedConn struct{ scopedConnector }

func (c scopedConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (c scopedConn) Close() error              { return nil }
func (c scopedConn) Begin() (driver.Tx, error) { return nil, errors.New("unexpected transaction") }
func (c scopedConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	*c.query = q
	*c.args = args
	return &accountRows{}, nil
}

func TestMessageSummarySQLScopesBeforeAggregation(t *testing.T) {
	var query string
	var args []driver.NamedValue
	previous := db
	conn := sql.OpenDB(scopedConnector{&query, &args})
	db = conn
	t.Cleanup(func() { db = previous; conn.Close() })
	for _, role := range []string{"customer", "barber"} {
		var e error
		if role == "customer" {
			_, e = ConversationsForCustomer(context.Background(), 42)
		} else {
			_, e = ConversationsForBarber(context.Background(), 42)
		}
		if e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(query, "where m."+role+"_id=$1") || len(args) != 1 || args[0].Value != int64(42) {
			t.Fatalf("SQL scope missing: %s %+v", query, args)
		}
	}
}
