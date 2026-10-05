package storage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"time"

	"barber-booking/internal/dbmigrate"
	"barber-booking/models"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	ErrNotFound   = errors.New("record not found")
	ErrEmailTaken = errors.New("email is already registered")
	ErrSlotTaken  = errors.New("that time slot is already booked")
	ErrForbidden  = errors.New("record belongs to someone else")
)

var (
	mu sync.RWMutex
	db *sql.DB

	barbers    []models.Barber
	customers  []models.Customer
	hairstyles []models.Hairstyle
	products   []models.Product
	bookings   []models.Booking
	messages   []models.Message
)

var (
	lastBarberID    int
	lastCustomerID  int
	lastHairstyleID int
	lastProductID   int
	lastBookingID   int
	lastMessageID   int
)

// InitDatabase enables Supabase/Postgres persistence when DATABASE_URL is set.
// Tests and local development without DATABASE_URL keep using the in-memory store.
func InitDatabase() error {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return nil
	}
	conn, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return err
	}
	if err := dbmigrate.Verify(ctx, conn); err != nil {
		conn.Close()
		db = nil
		return err
	}
	db = conn
	return nil
}

func CloseDatabase() error {
	if db == nil {
		return nil
	}
	err := db.Close()
	db = nil
	return err
}

func databaseEnabled() bool { return db != nil }

// Reset empties the in-memory test store. It intentionally does not delete
// persistent database records.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	barbers = nil
	customers = nil
	hairstyles = nil
	bookings = nil
	messages = nil
	products = nil
	carts = make(map[int]map[int]int)
	calendars = make(map[int]models.Calendar)
	productOrders = nil
	productOrderItems = nil
	lastProductOrderID = 0
	lastProductOrderItemID = 0
	lastBarberID = 0
	lastCustomerID = 0
	lastHairstyleID = 0
	lastBookingID = 0
	lastMessageID = 0
	lastProductID = 0
	reviews = nil
	lastReviewID = 0
}

func nextID(last *int) int { *last++; return *last }
