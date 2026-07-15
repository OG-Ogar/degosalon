package handlers

import (
	"net/http"
	"strings"
)

func AdminRouter(w http.ResponseWriter, r *http.Request) {

	path := r.URL.Path

	switch {

	case path == "/admin/login":
		AdminLoginHandler(w, r)

	case path == "/admin/dashboard":
		AdminDashboardHandler(w, r)

	case path == "/admin/barbers":
		ListBarbersHandler(w, r)

	case path == "/admin/barbers/new":
		NewBarberHandler(w, r)

	case strings.HasSuffix(path, "/edit"):
		EditBarberHandler(w, r)

	case strings.HasSuffix(path, "/update"):
		UpdateBarberHandler(w, r)

	case strings.HasSuffix(path, "/delete"):
		if r.Method == http.MethodPost {
			DeleteBarberHandler(w, r)
		} else {
			ConfirmDeleteBarberHandler(w, r)
		}

	default:
		BarberRouter(w, r)
	}
}
