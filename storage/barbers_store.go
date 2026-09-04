package storage

import (
	"strings"

	"barber-booking/models"
)

// ListBarbers returns every barber, newest last.
func ListBarbers() []models.Barber {
	mu.RLock()
	defer mu.RUnlock()

	return append([]models.Barber(nil), barbers...)
}

// ListActiveBarbers returns only barbers customers are allowed to see
// and book with.
func ListActiveBarbers() []models.Barber {
	mu.RLock()
	defer mu.RUnlock()

	var active []models.Barber

	for _, barber := range barbers {
		if barber.IsActive() {
			active = append(active, barber)
		}
	}

	return active
}

// FindBarberByID returns a copy of the barber with the given ID.
func FindBarberByID(id int) (models.Barber, bool) {
	mu.RLock()
	defer mu.RUnlock()

	return findBarberByID(id)
}

// findBarberByID is the unlocked form used by callers holding the lock.
func findBarberByID(id int) (models.Barber, bool) {
	for _, barber := range barbers {
		if barber.ID == id {
			return barber, true
		}
	}

	return models.Barber{}, false
}

// FindBarberByEmail matches case-insensitively, the way logins do.
func FindBarberByEmail(email string) (models.Barber, bool) {
	mu.RLock()
	defer mu.RUnlock()

	for _, barber := range barbers {
		if strings.EqualFold(barber.Email, email) {
			return barber, true
		}
	}

	return models.Barber{}, false
}

// CreateBarber assigns an ID and stores the barber. The email
// uniqueness check happens under the same lock as the insert, so two
// simultaneous registrations cannot both succeed with one email.
func CreateBarber(barber models.Barber) (models.Barber, error) {
	mu.Lock()
	defer mu.Unlock()

	for _, existing := range barbers {
		if strings.EqualFold(existing.Email, barber.Email) {
			return models.Barber{}, ErrEmailTaken
		}
	}

	barber.ID = nextID(&lastBarberID)

	if barber.Status == "" {
		barber.Status = models.BarberActive
	}

	barbers = append(barbers, barber)

	return barber, nil
}

// SetBarberPassword replaces one barber's stored password hash, in place
// and under a single lock, for the same reason SetCustomerPassword does.
func SetBarberPassword(id int, hash string) error {
	mu.Lock()
	defer mu.Unlock()

	for i := range barbers {
		if barbers[i].ID == id {
			barbers[i].PasswordHash = hash

			return nil
		}
	}

	return ErrNotFound
}

// UpdateBarber overwrites the stored barber that shares its ID. The
// password hash is preserved when the incoming copy leaves it empty, so
// profile edits cannot accidentally blank out a password.
func UpdateBarber(barber models.Barber) error {
	mu.Lock()
	defer mu.Unlock()

	for _, existing := range barbers {
		if existing.ID != barber.ID &&
			strings.EqualFold(existing.Email, barber.Email) {

			return ErrEmailTaken
		}
	}

	for i := range barbers {
		if barbers[i].ID != barber.ID {
			continue
		}

		if barber.PasswordHash == "" {
			barber.PasswordHash = barbers[i].PasswordHash
		}

		barbers[i] = barber

		return nil
	}

	return ErrNotFound
}

// SetBarberStatus activates or suspends an account.
func SetBarberStatus(id int, status string) error {
	mu.Lock()
	defer mu.Unlock()

	for i := range barbers {
		if barbers[i].ID == id {
			barbers[i].Status = status
			return nil
		}
	}

	return ErrNotFound
}

// DeleteBarber removes a barber together with their hairstyles,
// bookings and chat messages. It returns the on-disk image paths the
// caller should delete, because file removal is not the store's job.
func DeleteBarber(id int) ([]string, error) {
	mu.Lock()
	defer mu.Unlock()

	index := -1

	for i := range barbers {
		if barbers[i].ID == id {
			index = i
			break
		}
	}

	if index == -1 {
		return nil, ErrNotFound
	}

	var images []string
	var keptHairstyles []models.Hairstyle

	if barbers[index].ProfileImage != "" {
		images = append(images, barbers[index].ProfileImage)
	}

	for _, hairstyle := range hairstyles {
		if hairstyle.BarberID != id {
			keptHairstyles = append(keptHairstyles, hairstyle)
			continue
		}

		if hairstyle.Image != "" {
			images = append(images, hairstyle.Image)
		}
	}

	hairstyles = keptHairstyles

	var keptBookings []models.Booking

	for _, booking := range bookings {
		if booking.BarberID != id {
			keptBookings = append(keptBookings, booking)
		}
	}

	bookings = keptBookings

	var keptMessages []models.Message

	for _, message := range messages {
		if message.BarberID != id {
			keptMessages = append(keptMessages, message)
		}
	}

	messages = keptMessages
	barbers = append(barbers[:index], barbers[index+1:]...)

	return images, nil
}
