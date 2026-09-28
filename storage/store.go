// Package storage is the in-memory database for the application.
//
// Every table is a package-level slice guarded by one shared mutex, and
// every exported function returns copies rather than pointers into
// those slices. That combination is deliberate:
//
//   - net/http serves each request in its own goroutine, so unguarded
//     slice and map access is a data race.
//   - append may move a slice to a new backing array, which would leave
//     any previously handed-out pointer writing to the old array. Value
//     semantics plus explicit Update calls make writes obvious.
//
// Data lives only in memory and is lost when the process exits.
package storage

import (
	"errors"
	"sync"

	"barber-booking/models"
)

// Errors callers are expected to distinguish.
var (
	ErrNotFound   = errors.New("record not found")
	ErrEmailTaken = errors.New("email is already registered")
	ErrSlotTaken  = errors.New("that time slot is already booked")
	ErrForbidden  = errors.New("record belongs to someone else")
)

var (
	mu sync.RWMutex

	barbers    []models.Barber
	customers  []models.Customer
	hairstyles []models.Hairstyle
	products   []models.Product
	bookings   []models.Booking
	messages   []models.Message
)

// Highest ID handed out so far for each table. These only ever increase,
// so a deleted record's ID is never given to a new one.
var (
	lastBarberID    int
	lastCustomerID  int
	lastHairstyleID int
	lastProductID   int
	lastBookingID   int
	lastMessageID   int
)

// Reset empties every table. Tests call it to start from a clean slate.
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
}

// nextID hands out the next unused ID for one table. Callers must
// already hold the write lock.
//
// Deriving the ID from the records that are present is not safe: after
// the newest record is deleted the highest live ID drops, so the next
// record created inherits the deleted one's ID, and with it every
// booking, message and open session that still points at that ID.
func nextID(last *int) int {
	*last++

	return *last
}
