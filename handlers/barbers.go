package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"barber-booking/storage"
)

func BarberRouter(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {

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
		BarberProfileHandler(w, r)
	}
}

