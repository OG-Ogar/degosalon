package storage

import (
	"strings"
	"errors"

	"github.com/jackc/pgx/v5"

	"barber-booking/models"
)

// ListBarbers returns every barber, newest last.
func ListBarbers() []models.Barber {
	if databaseEnabled() {
		rows, err := db.Query("select id, full_name, phone_number, email, password_hash, shop_name, shop_address, state, local_area, profile_image, personal_image, status from public.barbers order by id")
		if err == nil {
			defer rows.Close()
			var out []models.Barber
			for rows.Next() {
				var b models.Barber
				if rows.Scan(&b.ID,&b.FullName,&b.PhoneNumber,&b.Email,&b.PasswordHash,&b.ShopName,&b.ShopAddress,&b.State,&b.LocalArea,&b.ProfileImage,&b.PersonalImage,&b.Status)==nil { out=append(out,b) }
			}
			return out
		}
	}
	mu.RLock()
	defer mu.RUnlock()

	return append([]models.Barber(nil), barbers...)
}

// ListActiveBarbers returns only barbers customers are allowed to see
// and book with.
func ListActiveBarbers() []models.Barber {
	if databaseEnabled() {
		rows, err := db.Query("select id, full_name, phone_number, email, password_hash, shop_name, shop_address, state, local_area, profile_image, personal_image, status from public.barbers where status='Active' order by id")
		if err == nil {
			defer rows.Close()
			var active []models.Barber
			for rows.Next() {
				var b models.Barber
				if rows.Scan(&b.ID,&b.FullName,&b.PhoneNumber,&b.Email,&b.PasswordHash,&b.ShopName,&b.ShopAddress,&b.State,&b.LocalArea,&b.ProfileImage,&b.PersonalImage,&b.Status)==nil { active=append(active,b) }
			}
			return active
		}
	}
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
	if databaseEnabled() {
		var b models.Barber
		err:=db.QueryRow("select id, full_name, phone_number, email, password_hash, shop_name, shop_address, state, local_area, profile_image, personal_image, status from public.barbers where id=$1",id).Scan(&b.ID,&b.FullName,&b.PhoneNumber,&b.Email,&b.PasswordHash,&b.ShopName,&b.ShopAddress,&b.State,&b.LocalArea,&b.ProfileImage,&b.PersonalImage,&b.Status)
		return b, err==nil
	}
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
	if databaseEnabled() {
		var b models.Barber
		err:=db.QueryRow("select id, full_name, phone_number, email, password_hash, shop_name, shop_address, state, local_area, profile_image, personal_image, status from public.barbers where lower(email)=lower($1)",email).Scan(&b.ID,&b.FullName,&b.PhoneNumber,&b.Email,&b.PasswordHash,&b.ShopName,&b.ShopAddress,&b.State,&b.LocalArea,&b.ProfileImage,&b.PersonalImage,&b.Status)
		return b, err==nil
	}
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
	if databaseEnabled() {
		if barber.Status=="" { barber.Status=models.BarberActive }
		err:=db.QueryRow("insert into public.barbers(full_name,phone_number,email,password_hash,shop_name,shop_address,state,local_area,profile_image,personal_image,status) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) returning id",barber.FullName,barber.PhoneNumber,barber.Email,barber.PasswordHash,barber.ShopName,barber.ShopAddress,barber.State,barber.LocalArea,barber.ProfileImage,barber.PersonalImage,barber.Status).Scan(&barber.ID)
		if err!=nil {
			if strings.Contains(strings.ToLower(err.Error()),"unique") { return models.Barber{},ErrEmailTaken }
			return models.Barber{},err
		}
		return barber,nil
	}
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
	if databaseEnabled() {
		result, err := db.Exec("update public.barbers set full_name=$1,phone_number=$2,email=$3,shop_name=$4,shop_address=$5,state=$6,local_area=$7,profile_image=$8,personal_image=$9,status=$10 where id=$11", barber.FullName,barber.PhoneNumber,barber.Email,barber.ShopName,barber.ShopAddress,barber.State,barber.LocalArea,barber.ProfileImage,barber.PersonalImage,barber.Status,barber.ID)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()),"unique") { return ErrEmailTaken }
			return err
		}
		n,_:=result.RowsAffected(); if n==0{return ErrNotFound}; return nil
	}
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

	for _, image := range []string{
		barbers[index].ProfileImage,
		barbers[index].PersonalImage,
	} {
		if image != "" {
			images = append(images, image)
		}
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

// FindBarberByIDResult distinguishes not-found from database failures.
func FindBarberByIDResult(id int) (models.Barber, error) {
	if databaseEnabled() {
		var v models.Barber
		err:=db.QueryRow("select id, full_name, phone_number, email, password_hash, shop_name, shop_address, state, local_area, profile_image, personal_image, status from public.barbers where id=$1",id).Scan(&v.ID,&v.FullName,&v.PhoneNumber,&v.Email,&v.PasswordHash,&v.ShopName,&v.ShopAddress,&v.State,&v.LocalArea,&v.ProfileImage,&v.PersonalImage,&v.Status)
		if errors.Is(err,pgx.ErrNoRows){return models.Barber{},ErrNotFound}
		return v,err
	}
	v,ok:=FindBarberByID(id);if !ok{return models.Barber{},ErrNotFound};return v,nil
}
