package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

type barberFormView struct {
	Editing bool
	ID      int
	Values  map[string]string
	CSRF    string
	Error   string
}

func renderBarberForm(w http.ResponseWriter, s sessionData, form barberForm, editingID int, problem string) {
	view := barberFormView{Editing: editingID > 0, ID: editingID, Values: form.values(), CSRF: s.CSRFToken, Error: problem}
	status := http.StatusOK
	if problem != "" {
		status = http.StatusBadRequest
	}
	renderStatus(w, status, "admin/edit-barber.html", view)
}

func EditBarberHandler(w http.ResponseWriter, r *http.Request, s sessionData, idSegment string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	id, ok := parseID(idSegment)
	if !ok {
		http.Error(w, "Invalid barber ID", http.StatusBadRequest)
		return
	}
	barber, err := storage.FindBarberByIDContext(r.Context(), id)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		serverError(w, "load admin barber", err)
		return
	}
	if errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	form := barberForm{
		FullName: barber.FullName, Phone: barber.PhoneNumber, Email: barber.Email,
		ShopName: barber.ShopName, ShopAddress: barber.ShopAddress,
		State: barber.State, LocalArea: barber.LocalArea,
	}
	renderBarberForm(w, s, form, barber.ID, "")
}

func UpdateBarberHandler(w http.ResponseWriter, r *http.Request, s sessionData, idSegment string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !requireCSRF(w, r, s) {
		return
	}
	id, ok := parseID(idSegment)
	if !ok {
		http.Error(w, "Invalid barber ID", http.StatusBadRequest)
		return
	}
	barber, err := storage.FindBarberByIDContext(r.Context(), id)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		serverError(w, "load admin barber", err)
		return
	}
	if errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	form, problem := readBarberForm(r)
	if problem != "" {
		renderBarberForm(w, s, form, id, problem)
		return
	}
	barber.FullName = form.FullName
	barber.PhoneNumber = form.Phone
	barber.Email = form.Email
	barber.ShopName = form.ShopName
	barber.ShopAddress = form.ShopAddress
	barber.State = form.State
	barber.LocalArea = form.LocalArea
	err = storage.UpdateBarber(barber)
	if errors.Is(err, storage.ErrEmailTaken) {
		renderBarberForm(w, s, form, id, "That email belongs to another barber.")
		return
	}
	if errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		serverError(w, "update barber", err)
		return
	}
	http.Redirect(w, r, "/admin/barbers/"+idSegment, http.StatusSeeOther)
}

func statusLabel(activate bool) string {
	if activate {
		return models.BarberActive
	}
	return models.BarberSuspended
}
