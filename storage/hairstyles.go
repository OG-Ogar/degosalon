package storage

import (
	"barber-booking/models"
	"context"
)

// ListHairstylesByBarber returns everything one barber offers.
func ListHairstylesByBarber(barberID int) []models.Hairstyle {
	if databaseEnabled() {
		rows, err := db.Query("select id,barber_id,name,category,description,price,duration,image,video,is_available from public.hairstyles where barber_id=$1 order by id", barberID)
		if err == nil {
			defer rows.Close()
			var out []models.Hairstyle
			for rows.Next() {
				var v models.Hairstyle
				if rows.Scan(&v.ID, &v.BarberID, &v.Name, &v.Category, &v.Description, &v.Price, &v.Duration, &v.Image, &v.Video, &v.IsAvailable) == nil {
					out = append(out, v)
				}
			}
			return out
		}
	}
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
	if databaseEnabled() {
		rows, err := db.Query("select id,barber_id,name,category,description,price,duration,image,video,is_available from public.hairstyles where barber_id=$1 and is_available=true order by id", barberID)
		if err == nil {
			defer rows.Close()
			var out []models.Hairstyle
			for rows.Next() {
				var v models.Hairstyle
				if rows.Scan(&v.ID, &v.BarberID, &v.Name, &v.Category, &v.Description, &v.Price, &v.Duration, &v.Image, &v.Video, &v.IsAvailable) == nil {
					out = append(out, v)
				}
			}
			return out
		}
	}
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
	if databaseEnabled() {
		var v models.Hairstyle
		err := db.QueryRow("select id,barber_id,name,category,description,price,duration,image,video,is_available from public.hairstyles where id=$1", id).Scan(&v.ID, &v.BarberID, &v.Name, &v.Category, &v.Description, &v.Price, &v.Duration, &v.Image, &v.Video, &v.IsAvailable)
		return v, err == nil
	}
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
func FindHairstyleOwnedBy(ctx context.Context, id, barberID int) (models.Hairstyle, error) {
	v, err := FindHairstyleByIDResult(ctx, id)
	if err != nil {
		return models.Hairstyle{}, err
	}
	if v.BarberID != barberID {
		return models.Hairstyle{}, ErrForbidden
	}
	return v, nil
}

// CreateHairstyle assigns an ID and stores the hairstyle.
func CreateHairstyle(hairstyle models.Hairstyle) (models.Hairstyle, error) {
	if databaseEnabled() {
		err := db.QueryRow("insert into public.hairstyles(barber_id,name,category,description,price,duration,image,video,is_available) values($1,$2,$3,$4,$5,$6,$7,$8,$9) returning id", hairstyle.BarberID, hairstyle.Name, hairstyle.Category, hairstyle.Description, hairstyle.Price, hairstyle.Duration, hairstyle.Image, hairstyle.Video, hairstyle.IsAvailable).Scan(&hairstyle.ID)
		return hairstyle, err
	}
	mu.Lock()
	defer mu.Unlock()
	if err := guardMediaLocked(hairstyle.Image, hairstyle.Video); err != nil {
		return models.Hairstyle{}, err
	}

	hairstyle.ID = nextID(&lastHairstyleID)

	hairstyles = append(hairstyles, hairstyle)

	return hairstyle, nil
}

// UpdateHairstyle overwrites the stored hairstyle that shares its ID,
// refusing to write across barbers.
func UpdateHairstyle(hairstyle models.Hairstyle) error {
	if databaseEnabled() {
		result, err := db.Exec("update public.hairstyles set name=$1,category=$2,description=$3,price=$4,duration=$5,image=$6,video=$7,is_available=$8 where id=$9 and barber_id=$10", hairstyle.Name, hairstyle.Category, hairstyle.Description, hairstyle.Price, hairstyle.Duration, hairstyle.Image, hairstyle.Video, hairstyle.IsAvailable, hairstyle.ID, hairstyle.BarberID)
		if err != nil {
			return err
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			return ErrNotFound
		}
		return nil
	}
	mu.Lock()
	defer mu.Unlock()

	for i := range hairstyles {
		if hairstyles[i].ID != hairstyle.ID {
			continue
		}

		if hairstyles[i].BarberID != hairstyle.BarberID {
			return ErrForbidden
		}

		if err := guardMediaLocked(hairstyle.Image, hairstyle.Video); err != nil {
			return err
		}
		hairstyles[i] = hairstyle

		return nil
	}

	return ErrNotFound
}

// DeleteHairstyle removes one hairstyle owned by barberID and returns
// its image path so the caller can delete the file from disk.
func DeleteHairstyle(id, barberID int) (string, error) {
	if databaseEnabled() {
		var image string
		err := db.QueryRow("delete from public.hairstyles where id=$1 and barber_id=$2 returning image", id, barberID).Scan(&image)
		if err != nil {
			return "", recordError(err)
		}
		return image, nil
	}
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

// FindHairstyleByIDResult distinguishes not-found from database failures.
func FindHairstyleByIDResult(ctx context.Context, id int) (models.Hairstyle, error) {
	if err := ctx.Err(); err != nil {
		return models.Hairstyle{}, err
	}
	if !databaseEnabled() {
		v, ok := FindHairstyleByID(id)
		if !ok {
			return models.Hairstyle{}, ErrNotFound
		}
		return v, nil
	}
	var v models.Hairstyle
	err := db.QueryRowContext(ctx, "select id,barber_id,name,category,description,price,duration,image,video,is_available from public.hairstyles where id=$1", id).Scan(&v.ID, &v.BarberID, &v.Name, &v.Category, &v.Description, &v.Price, &v.Duration, &v.Image, &v.Video, &v.IsAvailable)
	return v, recordError(err)
}
