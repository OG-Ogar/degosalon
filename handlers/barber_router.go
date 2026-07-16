package handlers

import (
	"net/http"
	"strings"
)

func BarberRouter(w http.ResponseWriter, r *http.Request) {

	path := r.URL.Path

	switch {

	case path == "/barber/login":
		BarberLoginHandler(w, r)

	case path == "/barber/dashboard":
		BarberDashboardHandler(w, r)

	case path == "/barber/hairstyles":
		ListHairstylesHandler(w, r)

	case path == "/barber/hairstyles/new":
		NewHairstyleHandler(w, r)

	case strings.HasSuffix(path, "/edit"):
		EditHairstyleHandler(w, r)

	case strings.HasSuffix(path, "/update"):
		UpdateHairstyleHandler(w, r)

	case strings.HasSuffix(path, "/delete"):

		if r.Method == http.MethodPost {
			DeleteHairstyleHandler(w, r)
		} else {
			ConfirmDeleteHairstyleHandler(w, r)
		}

	default:
		ViewHairstyleHandler(w, r)
	}
}
