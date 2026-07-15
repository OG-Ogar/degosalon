package handlers

import "net/http"

func BarberRouter(w http.ResponseWriter, r *http.Request) {

	switch r.URL.Path {

	case "/barber/login":
		BarberLoginHandler(w, r)

	case "/barber/dashboard":
		BarberDashboardHandler(w, r)

	default:
		http.NotFound(w, r)
	}
}