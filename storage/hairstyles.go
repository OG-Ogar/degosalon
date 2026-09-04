package storage

import "barber-booking/models"

// ListHairstylesByBarber returns everything one barber offers.
func ListHairstylesByBarber(barberID int) []models.Hairstyle {
	mu.RLock()
	defer mu.RUnlock()

	var owned []models.Hairstyle

	for _, hairstyle := range hairstyles {
		if hairstyle.BarberID == barberID {
			owned = append(owned, hairstyle)
		}
	}

	return owned
}

// ListAvailableHairstylesByBarber returns only the hairstyles a
// customer is allowed to book.
func ListAvailableHairstylesByBarber(barberID int) []models.Hairstyle {
	mu.RLock()
	defer mu.RUnlock()

	var available []models.Hairstyle

	for _, hairstyle := range hairstyles {
		if hairstyle.BarberID == barberID && hairstyle.IsAvailable {
			available = append(available, hairstyle)
		}
	}

	return available
}

// FindHairstyleByID returns a copy of the hairstyle with the given ID.
func FindHairstyleByID(id int) (models.Hairstyle, bool) {
	mu.RLock()
	defer mu.RUnlock()

	return findHairstyleByID(id)
}

// findHairstyleByID is the unlocked form used by callers holding the lock.
func findHairstyleByID(id int) (models.Hairstyle, bool) {
	for _, hairstyle := range hairstyles {
		if hairstyle.ID == id {
			return hairstyle, true
		}
	}

	return models.Hairstyle{}, false
}

// FindHairstyleOwnedBy returns the hairstyle only when barberID owns it,
// so a barber cannot reach another barber's records by guessing IDs.
func FindHairstyleOwnedBy(id, barberID int) (models.Hairstyle, error) {
	mu.RLock()
	defer mu.RUnlock()

	hairstyle, found := findHairstyleByID(id)
	if !found {
		return models.Hairstyle{}, ErrNotFound
	}

	if hairstyle.BarberID != barberID {
		return models.Hairstyle{}, ErrForbidden
	}

	return hairstyle, nil
}

// CreateHairstyle assigns an ID and stores the hairstyle.
func CreateHairstyle(hairstyle models.Hairstyle) (models.Hairstyle, error) {
	mu.Lock()
	defer mu.Unlock()

	hairstyle.ID = nextID(&lastHairstyleID)

	hairstyles = append(hairstyles, hairstyle)

	return hairstyle, nil
}

// UpdateHairstyle overwrites the stored hairstyle that shares its ID,
// refusing to write across barbers.
func UpdateHairstyle(hairstyle models.Hairstyle) error {
	mu.Lock()
	defer mu.Unlock()

	for i := range hairstyles {
		if hairstyles[i].ID != hairstyle.ID {
			continue
		}

		if hairstyles[i].BarberID != hairstyle.BarberID {
			return ErrForbidden
		}

		hairstyles[i] = hairstyle

		return nil
	}

	return ErrNotFound
}

// DeleteHairstyle removes one hairstyle owned by barberID and returns
// its image path so the caller can delete the file from disk.
func DeleteHairstyle(id, barberID int) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	for i := range hairstyles {
		if hairstyles[i].ID != id {
			continue
		}

		if hairstyles[i].BarberID != barberID {
			return "", ErrForbidden
		}

		image := hairstyles[i].Image

		hairstyles = append(hairstyles[:i], hairstyles[i+1:]...)

		return image, nil
	}

	return "", ErrNotFound
}
