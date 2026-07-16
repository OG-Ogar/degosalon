package storage

import "barber-booking/models"

var Barbers []models.Barber

func FindBarberByID(id int) (*models.Barber, bool) {
	for i := range Barbers {
		if Barbers[i].ID == id {
			return &Barbers[i], true
		}
	}
	return nil, false
}
