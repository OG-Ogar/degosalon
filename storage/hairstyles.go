package storage

import (
	"barber-booking/internal/money"
	"barber-booking/models"
	"context"
)

// ListHairstylesByBarber returns everything one barber offers.
func ListHairstylesByBarber(barberID int) []models.Hairstyle {
	ctx, cancel := databaseContext(context.Background())
	defer cancel()

	if databaseEnabled() {
		rows, err := db.QueryContext(ctx, "select id,barber_id,name,category,description,price,duration,image,video,is_available,is_archived from public.hairstyles where barber_id=$1 and not is_archived order by id", barberID)
		if err == nil {
			defer rows.Close()
			var out []models.Hairstyle
			for rows.Next() {
				var v models.Hairstyle
				if rows.Scan(&v.ID, &v.BarberID, &v.Name, &v.Category, &v.Description, &v.Price, &v.Duration, &v.Image, &v.Video, &v.IsAvailable, &v.Archived) == nil {
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
		if hairstyle.BarberID == barberID && !hairstyle.Archived {
			owned = append(owned, hairstyle)
		}
	}

	return owned
}

// ListAvailableHairstylesByBarber returns only the hairstyles a
// customer is allowed to book.
func ListAvailableHairstylesByBarber(barberID int) []models.Hairstyle {
	ctx, cancel := databaseContext(context.Background())
	defer cancel()

	if databaseEnabled() {
		rows, err := db.QueryContext(ctx, "select id,barber_id,name,category,description,price,duration,image,video,is_available,is_archived from public.hairstyles where barber_id=$1 and is_available=true and not is_archived order by id", barberID)
		if err == nil {
			defer rows.Close()
			var out []models.Hairstyle
			for rows.Next() {
				var v models.Hairstyle
				if rows.Scan(&v.ID, &v.BarberID, &v.Name, &v.Category, &v.Description, &v.Price, &v.Duration, &v.Image, &v.Video, &v.IsAvailable, &v.Archived) == nil {
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
		if hairstyle.BarberID == barberID && hairstyle.IsAvailable && !hairstyle.Archived {
			available = append(available, hairstyle)
		}
	}

	return available
}

// FindHairstyleByID returns a copy of the hairstyle with the given ID.
func FindHairstyleByID(id int) (models.Hairstyle, bool) {
	ctx, cancel := databaseContext(context.Background())
	defer cancel()

	if databaseEnabled() {
		var v models.Hairstyle
		err := db.QueryRowContext(ctx, "select id,barber_id,name,category,description,price,duration,image,video,is_available,is_archived from public.hairstyles where id=$1", id).Scan(&v.ID, &v.BarberID, &v.Name, &v.Category, &v.Description, &v.Price, &v.Duration, &v.Image, &v.Video, &v.IsAvailable, &v.Archived)
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
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	v, err := FindHairstyleByIDResult(ctx, id)
	if err != nil {
		return models.Hairstyle{}, err
	}
	if v.Archived {
		return models.Hairstyle{}, ErrNotFound
	}
	if v.BarberID != barberID {
		return models.Hairstyle{}, ErrForbidden
	}
	return v, nil
}

// CreateHairstyle assigns an ID and stores the hairstyle.
func CreateHairstyle(hairstyle models.Hairstyle) (models.Hairstyle, error) {
	return CreateHairstyleContext(context.Background(), hairstyle)
}

func CreateHairstyleContext(ctx context.Context, hairstyle models.Hairstyle) (models.Hairstyle, error) {
	hairstyle.Archived = false
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return models.Hairstyle{}, err
	}

	if _, err := money.FromFloat(hairstyle.Price); err != nil {
		return models.Hairstyle{}, err
	}

	if databaseEnabled() {
		err := db.QueryRowContext(ctx, "insert into public.hairstyles(barber_id,name,category,description,price,duration,image,video,is_available) values($1,$2,$3,$4,$5,$6,$7,$8,$9) returning id", hairstyle.BarberID, hairstyle.Name, hairstyle.Category, hairstyle.Description, hairstyle.Price, hairstyle.Duration, hairstyle.Image, hairstyle.Video, hairstyle.IsAvailable).Scan(&hairstyle.ID)
		return hairstyle, err
	}
	mu.Lock()
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return models.Hairstyle{}, err
	}
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
	return UpdateHairstyleContext(context.Background(), hairstyle)
}

func UpdateHairstyleContext(ctx context.Context, hairstyle models.Hairstyle) error {
	hairstyle.Archived = false
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}

	if _, err := money.FromFloat(hairstyle.Price); err != nil {
		return err
	}

	if databaseEnabled() {
		result, err := db.ExecContext(ctx, "update public.hairstyles set name=$1,category=$2,description=$3,price=$4,duration=$5,image=$6,video=$7,is_available=$8 where id=$9 and barber_id=$10 and not is_archived", hairstyle.Name, hairstyle.Category, hairstyle.Description, hairstyle.Price, hairstyle.Duration, hairstyle.Image, hairstyle.Video, hairstyle.IsAvailable, hairstyle.ID, hairstyle.BarberID)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}

	for i := range hairstyles {
		if hairstyles[i].ID != hairstyle.ID {
			continue
		}

		if hairstyles[i].Archived {
			return ErrNotFound
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

// DeleteHairstyle archives an owned hairstyle, retaining references and media.
// The compatibility return path is empty because no media may be removed.
func DeleteHairstyle(id, barberID int) (string, error) {
	return DeleteHairstyleContext(context.Background(), id, barberID)
}

func DeleteHairstyleContext(ctx context.Context, id, barberID int) (string, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if databaseEnabled() {
		var archivedID int
		err := db.QueryRowContext(ctx, "update public.hairstyles set is_archived=true,is_available=false where id=$1 and barber_id=$2 and not is_archived returning id", id, barberID).Scan(&archivedID)
		if err != nil {
			return "", recordError(err)
		}
		return "", nil
	}
	mu.Lock()
	defer mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}

	for i := range hairstyles {
		if hairstyles[i].ID != id {
			continue
		}

		if hairstyles[i].Archived {
			return "", ErrNotFound
		}
		if hairstyles[i].BarberID != barberID {
			return "", ErrForbidden
		}

		hairstyles[i].Archived = true
		hairstyles[i].IsAvailable = false
		return "", nil
	}

	return "", ErrNotFound
}

// FindHairstyleByIDResult distinguishes not-found from database failures.
func FindHairstyleByIDResult(ctx context.Context, id int) (models.Hairstyle, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

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
	err := db.QueryRowContext(ctx, "select id,barber_id,name,category,description,price,duration,image,video,is_available,is_archived from public.hairstyles where id=$1", id).Scan(&v.ID, &v.BarberID, &v.Name, &v.Category, &v.Description, &v.Price, &v.Duration, &v.Image, &v.Video, &v.IsAvailable, &v.Archived)
	return v, recordError(err)
}
