package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

// barberFormView backs the admin form that both creates and edits a
// barber. Editing switches the heading, the action URL and whether a
// password field is shown.
type barberFormView struct {
	Editing bool
	ID      int

	Values map[string]string
	CSRF   string
	Error  string
}

// renderBarberForm shows the add/edit form, filled in with whatever was
// submitted so a rejected attempt does not lose the visitor's typing.
func renderBarberForm(
	w http.ResponseWriter,
	s sessionData,
	form barberForm,
	editingID int,
	problem string,
) {
	view := barberFormView{
		Editing: editingID > 0,
		ID:      editingID,
		Values:  form.values(),
		CSRF:    s.CSRFToken,
		Error:   problem,
	}

	status := http.StatusOK

	if problem != "" {
		status = http.StatusBadRequest
	}

	renderStatus(w, status, "admin/edit-barber.html", view)
}

// EditBarberHandler shows the edit form for one barber.
//
// It previously parsed templates/admin/barber-form.html, a file that does
// not exist, so this route returned 500 on every visit.
func EditBarberHandler(
	w http.ResponseWriter,
	r *http.Request,
	s sessionData,
	idSegment string,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	id, ok := parseID(idSegment)
	if !ok {
		http.Error(w, "Invalid barber ID", http.StatusBadRequest)

		return
	}

	barber, found := storage.FindBarberByID(id)
	if !found {
		http.NotFound(w, r)

		return
	}

	form := barberForm{
		FullName:    barber.FullName,
		Phone:       barber.PhoneNumber,
		Email:       barber.Email,
		ShopName:    barber.ShopName,
		ShopAddress: barber.ShopAddress,
	}

	renderBarberForm(w, s, form, barber.ID, "")
}

// UpdateBarberHandler saves an edited barber.
func UpdateBarberHandler(
	w http.ResponseWriter,
	r *http.Request,
	s sessionData,
	idSegment string,
) {
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

	barber, found := storage.FindBarberByID(id)
	if !found {
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

	err := storage.UpdateBarber(barber)

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

// statusLabel converts the activate flag into a stored status.
func statusLabel(activate bool) string {
	if activate {
		return models.BarberActive
	}

	return models.BarberSuspended
}
