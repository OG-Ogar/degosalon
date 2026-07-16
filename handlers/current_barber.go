package handlers

import (
	"barber-booking/models"
	"barber-booking/session"
	"barber-booking/storage"
	"net/http"
)

func CurrentBarber(r *http.Request) (*models.Barber, bool) {

	cookie, err := r.Cookie("barber_session")
	if err != nil {
		return nil, false
	}

	s, found := session.GetSession(cookie.Value)
	if !found {
		return nil, false
	}

	barber, found := storage.FindBarberByID(s.BarberID)
	if !found {
		return nil, false
	}

	return barber, true
}
