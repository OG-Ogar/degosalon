package storage

import (
	"barber-booking/internal/testdb"
	"barber-booking/models"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func checkMessagePages(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	seller := newBarber(t, "message-page-owner@example.test")
	var buyers []models.Customer
	for i := 0; i < 26; i++ {
		buyers = append(buyers, newCustomer(t, fmt.Sprintf("message-page-%d@example.test", i)))
	}
	send := func(barber, customer int, text string) models.Message {
		m, err := AddMessage(ctx, models.Message{BarberID: barber, CustomerID: customer, SenderRole: "customer", SenderName: "Sender", Text: text, SentAt: time.Now()})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	for _, buyer := range buyers[1:] {
		send(seller.ID, buyer.ID, "other owned conversation")
	}
	var thread []models.Message
	for i := 0; i < 130; i++ {
		thread = append(thread, send(seller.ID, buyers[0].ID, fmt.Sprintf("history %03d", i)))
	}
	var otherSellers []models.Barber
	for i := 0; i < 25; i++ {
		b := newBarber(t, fmt.Sprintf("message-peer-%d@example.test", i))
		otherSellers = append(otherSellers, b)
		send(b.ID, buyers[0].ID, "other seller conversation")
	}
	send(otherSellers[0].ID, buyers[1].ID, "private other pair")
	latest, err := MessagePage(ctx, seller.ID, buyers[0].ID, PageCursor{Limit: 100})
	if err != nil || len(latest.Items) != 100 || latest.HasPrevious || !latest.HasNext || latest.Items[0].ID != thread[129].ID || latest.Items[99].ID != thread[30].ID {
		t.Fatalf("latest thread page: %+v %v", latest, err)
	}
	older, err := MessagePage(ctx, seller.ID, buyers[0].ID, PageCursor{Before: latest.Items[99].ID, Limit: 100})
	if err != nil || len(older.Items) != 30 || !older.HasPrevious || older.HasNext || older.Items[29].ID != thread[0].ID {
		t.Fatalf("older thread page: %+v %v", older, err)
	}
	newer, err := MessagePage(ctx, seller.ID, buyers[0].ID, PageCursor{After: older.Items[0].ID, Limit: 100})
	if err != nil || len(newer.Items) != 100 || newer.Items[0].ID != thread[129].ID || newer.HasPrevious || !newer.HasNext {
		t.Fatalf("newer thread page: %+v %v", newer, err)
	}
	reply := send(seller.ID, buyers[0].ID, "new reply")
	unchanged, err := MessagePage(ctx, seller.ID, buyers[0].ID, PageCursor{Before: latest.Items[99].ID, Limit: 100})
	if err != nil || len(unchanged.Items) != 30 || unchanged.Items[0].ID != older.Items[0].ID {
		t.Fatal("new reply shifted old history boundary")
	}
	refreshed, err := MessagePage(ctx, seller.ID, buyers[0].ID, PageCursor{Limit: 100})
	if err != nil || refreshed.Items[0].ID != reply.ID {
		t.Fatal("refresh missed latest reply")
	}
	for _, role := range []string{"customer", "barber"} {
		var page Page[ConversationCard]
		var count int
		read := func(cursor PageCursor) (Page[ConversationCard], error) {
			if role == "customer" {
				return ConversationPageForCustomer(ctx, buyers[0].ID, cursor)
			}
			return ConversationPageForBarber(ctx, seller.ID, cursor)
		}
		page, err = read(PageCursor{})
		if err != nil || len(page.Items) != 24 || !page.HasNext || page.HasPrevious {
			t.Fatalf("%s list: %+v %v", role, page, err)
		}
		if page.Items[0].LastID != reply.ID || page.Items[0].Count != 131 || page.Items[0].Name == "" {
			t.Fatalf("latest participant summary wrong: %+v", page.Items[0])
		}
		for _, c := range page.Items {
			if role == "customer" && c.CustomerID != buyers[0].ID || role == "barber" && c.BarberID != seller.ID {
				t.Fatal("conversation ownership leaked")
			}
		}
		next, err := read(PageCursor{Before: page.Items[23].LastID})
		if err != nil || len(next.Items) != 2 || next.HasNext || !next.HasPrevious {
			t.Fatalf("%s next list: %+v %v", role, next, err)
		}
		back, err := read(PageCursor{After: next.Items[0].LastID})
		if err != nil || len(back.Items) != 24 || back.Items[0].LastID != page.Items[0].LastID {
			t.Fatal("conversation backward page failed")
		}
		if role == "customer" {
			count, err = ConversationCountForCustomer(ctx, buyers[0].ID)
		} else {
			count, err = ConversationCountForBarber(ctx, seller.ID)
		}
		if err != nil || count != 26 {
			t.Fatalf("%s aggregate count=%d err=%v", role, count, err)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := MessagePage(canceled, seller.ID, buyers[0].ID, PageCursor{}); !errors.Is(err, context.Canceled) {
		t.Fatal("message cancellation ignored")
	}
	if _, err := ConversationPageForCustomer(canceled, buyers[0].ID, PageCursor{}); !errors.Is(err, context.Canceled) {
		t.Fatal("list cancellation ignored")
	}
}
func TestMessagePages(t *testing.T) { Reset(); t.Cleanup(Reset); checkMessagePages(t) }
func TestPostgresMessagePages(t *testing.T) {
	address := testdb.New(t, "../supabase/schema.sql")
	connection, err := sql.Open("pgx", address)
	if err != nil {
		t.Fatal(err)
	}
	previous := db
	db = connection
	t.Cleanup(func() { db = previous; connection.Close() })
	checkMessagePages(t)
}
func TestMessagePageQueriesAreScopedAndBounded(t *testing.T) {
	var query string
	var args []driver.NamedValue
	connection := sql.OpenDB(scopedConnector{&query, &args})
	previous := db
	db = connection
	t.Cleanup(func() { db = previous; connection.Close() })
	if _, err := MessagePage(context.Background(), 42, 55, PageCursor{Limit: 100, Before: 99}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query, "where barber_id=$1 and customer_id=$2") || !strings.Contains(query, "id<$3") || !strings.Contains(query, "limit $4") || len(args) != 4 || args[3].Value != int64(101) {
		t.Fatalf("unbounded thread: %s %v", query, args)
	}
	for _, role := range []string{"customer", "barber"} {
		var err error
		if role == "customer" {
			_, err = ConversationPageForCustomer(context.Background(), 42, PageCursor{})
		} else {
			_, err = ConversationPageForBarber(context.Background(), 42, PageCursor{})
		}
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(query, "where m."+role+"_id=$1") || !strings.Contains(query, "page as materialized") || !strings.Contains(query, "limit $2") || len(args) != 2 || args[1].Value != int64(25) {
			t.Fatalf("unbounded list: %s %v", query, args)
		}
		if strings.Contains(query, "password_hash") || strings.Contains(query, "email") {
			t.Fatal("private account fields selected")
		}
	}
}
func TestMessagePageDatabaseFailuresPropagate(t *testing.T) {
	failure := errors.New("message page outage")
	connection := sql.OpenDB(accountConnector{queryErr: failure})
	previous := db
	db = connection
	t.Cleanup(func() { db = previous; connection.Close() })
	for name, read := range map[string]func() error{
		"thread":        func() error { _, e := MessagePage(context.Background(), 1, 1, PageCursor{}); return e },
		"customer list": func() error { _, e := ConversationPageForCustomer(context.Background(), 1, PageCursor{}); return e },
		"barber list":   func() error { _, e := ConversationPageForBarber(context.Background(), 1, PageCursor{}); return e },
		"count":         func() error { _, e := ConversationCountForCustomer(context.Background(), 1); return e },
	} {
		if err := read(); !errors.Is(err, failure) {
			t.Fatalf("%s outage hidden: %v", name, err)
		}
	}
}
