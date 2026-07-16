package storage

import "barber-booking/models"

var Hairstyles []models.Hairstyle

func FindHairstyleByID(id int) (*models.Hairstyle, bool) {

	for i := range Hairstyles {

		if Hairstyles[i].ID == id {

			return &Hairstyles[i], true

		}

	}

	return nil, false
}
