package storage

import (
	"errors"
	"sort"

	"barber-booking/models"
)

// ErrUnavailable is returned when the requested hairstyle exists but is
// not open for booking, or the barber is suspended.
var ErrUnavailable = errors.New("that hairstyle is not available for booking")

// CreateBooking validates and stores a new booking.
//
// The barber, hairstyle, availability and slot-conflict checks all run
// inside one write lock, so two customers submitting the same slot at
// the same moment cannot both be accepted.
func CreateBooking(booking models.Booking) (models.Booking, error) {
	mu.Lock()
	defer mu.Unlock()

	barber, found := findBarberByID(booking.BarberID)
	if !found {
		return models.Booking{}, ErrNotFound
	}

	if !barber.IsActive() {
		return models.Booking{}, ErrUnavailable
	}

	hairstyle, found := findHairstyleByID(booking.HairstyleID)
	if !found {
		return models.Booking{}, ErrNotFound
	}

	if hairstyle.BarberID != booking.BarberID {
		return models.Booking{}, ErrForbidden
	}

	if !hairstyle.IsAvailable {
		return models.Booking{}, ErrUnavailable
	}

	for _, existing := range bookings {
		if existing.BarberID == booking.BarberID &&
			existing.Date == booking.Date &&
			existing.Time == booking.Time &&
			existing.IsOpen() {

			return models.Booking{}, ErrSlotTaken
		}
	}

	booking.ID = nextID(&lastBookingID)

	bookings = append(bookings, booking)

	return booking, nil
}

// FindBookingByID returns a copy of the booking with the given ID.
func FindBookingByID(id int) (models.Booking, bool) {
	mu.RLock()
	defer mu.RUnlock()

	for _, booking := range bookings {
		if booking.ID == id {
			return booking, true
		}
	}

	return models.Booking{}, false
}

// ListBookings returns every booking, for the admin overview.
func ListBookings() []models.Booking {
	mu.RLock()
	defer mu.RUnlock()

	return sortBySlot(append([]models.Booking(nil), bookings...))
}

// ListBookingsByBarber returns one barber's schedule.
func ListBookingsByBarber(barberID int) []models.Booking {
	mu.RLock()
	defer mu.RUnlock()

	var owned []models.Booking

	for _, booking := range bookings {
		if booking.BarberID == barberID {
			owned = append(owned, booking)
		}
	}

	return sortBySlot(owned)
}

// ListBookingsByCustomer returns one customer's appointments.
func ListBookingsByCustomer(customerID int) []models.Booking {
	mu.RLock()
	defer mu.RUnlock()

	var owned []models.Booking

	for _, booking := range bookings {
		if booking.CustomerID == customerID {
			owned = append(owned, booking)
		}
	}

	return sortBySlot(owned)
}

// SetBookingStatus moves a booking to a new status. The caller passes
// the ID it is allowed to act on: barberID for the barber's own
// schedule, customerID for the customer's own appointments. Passing 0
// skips that check, which only the admin does.
func SetBookingStatus(id int, status string, barberID, customerID int) error {
	mu.Lock()
	defer mu.Unlock()

	for i := range bookings {
		if bookings[i].ID != id {
			continue
		}

		if barberID != 0 && bookings[i].BarberID != barberID {
			return ErrForbidden
		}

		if customerID != 0 && bookings[i].CustomerID != customerID {
			return ErrForbidden
		}

		bookings[i].Status = status

		return nil
	}

	return ErrNotFound
}

// sortBySlot orders bookings by appointment date then time, earliest
// first, so both dashboards read like a schedule.
func sortBySlot(list []models.Booking) []models.Booking {
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Date != list[j].Date {
			return list[i].Date < list[j].Date
		}

		if list[i].Time != list[j].Time {
			return list[i].Time < list[j].Time
		}

		return list[i].ID < list[j].ID
	})

	return list
}
