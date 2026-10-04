package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/security"
	"barber-booking/storage"
)

// barberForm is the profile data the admin add/edit form submits.
type barberForm struct {
	FullName    string
	Phone       string
	Email       string
	ShopName    string
	ShopAddress string
	State       string
	LocalArea   string
}

// readBarberForm pulls the fields out of a request and returns a
// validation message when something is missing.
func readBarberForm(r *http.Request) (barberForm, string) {
	form := barberForm{
		FullName:    trimmedField(r.PostFormValue("fullname")),
		Phone:       trimmedField(r.PostFormValue("phone")),
		Email:       normalisedEmail(r.PostFormValue("email")),
		ShopName:    trimmedField(r.PostFormValue("shopname")),
		ShopAddress: trimmedField(r.PostFormValue("shopaddress")),
		State:       trimmedField(r.PostFormValue("state")),
		LocalArea:   trimmedField(r.PostFormValue("local_area")),
	}

	if form.FullName == "" ||
		form.Phone == "" ||
		form.Email == "" ||
		form.ShopName == "" ||
		form.ShopAddress == "" {

		return form, "All fields are required."
	}

	return form, ""
}

// values renders the form back into the map the template reads.
func (f barberForm) values() map[string]string {
	return map[string]string{
		"fullname":    f.FullName,
		"phone":       f.Phone,
		"email":       f.Email,
		"shopname":    f.ShopName,
		"shopaddress": f.ShopAddress,
		"state":       f.State,
		"local_area":  f.LocalArea,
	}
}

// ListBarbersHandler shows the barber table.
//
// It builds rows without the password hash rather than handing the
// stored records straight to the template.
func ListBarbersHandler(w http.ResponseWriter, r *http.Request, s sessionData) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	barbers := storage.ListBarbers()
	rows := make([]adminBarberRow, 0, len(barbers))

	for _, barber := range barbers {
		rows = append(rows, adminBarberRow{
			ID:          barber.ID,
			FullName:    barber.FullName,
			PhoneNumber: barber.PhoneNumber,
			Email:       barber.Email,
			ShopName:    barber.ShopName,
			Status:      barber.Status,
			Active:      barber.IsActive(),
			Hairstyles:  len(storage.ListHairstylesByBarber(barber.ID)),
			Bookings:    len(storage.ListBookingsByBarber(barber.ID)),
		})
	}

	render(w, "admin/barber-list.html", listView[adminBarberRow]{
		Items: rows,
		CSRF:  s.CSRFToken,
	})
}

// adminBarberRow is one line of the barber table. It deliberately has no
// password field.
type adminBarberRow struct {
	ID          int
	FullName    string
	PhoneNumber string
	Email       string
	ShopName    string
	Status      string
	Active      bool
	Hairstyles  int
	Bookings    int
}

// adminBarberProfileView is one barber with their hairstyles and
// bookings, for the admin detail page.
type adminBarberProfileView struct {
	Barber     models.Barber
	Hairstyles []models.Hairstyle
	Bookings   []bookingRow

	CSRF string
}

// BarberProfileHandler shows one barber in full.
func BarberProfileHandler(
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

	barber.PasswordHash = ""

	render(w, "admin/barber-profile.html", adminBarberProfileView{
		Barber:     barber,
		Hairstyles: storage.ListHairstylesByBarber(id),
		Bookings:   describeBookings(storage.ListBookingsByBarber(id)),
		CSRF:       s.CSRFToken,
	})
}

// NewBarberHandler lets the admin create a barber account, the flow the
// README describes but which had no route.
func NewBarberHandler(w http.ResponseWriter, r *http.Request, s sessionData) {
	switch r.Method {

	case http.MethodGet:
		render(w, "admin/edit-barber.html", barberFormView{
			CSRF:   s.CSRFToken,
			Values: map[string]string{},
		})

	case http.MethodPost:
		if !requireCSRF(w, r, s) {
			return
		}

		form, problem := readBarberForm(r)
		password := r.PostFormValue("password")

		if problem == "" && len(password) < minPasswordLength {
			problem = passwordTooShortMessage
		}

		if problem != "" {
			renderBarberForm(w, s, form, 0, problem)

			return
		}

		hash, err := security.HashPassword(password)
		if err != nil {
			serverError(w, "hash barber password", err)

			return
		}

		_, err = storage.CreateBarber(models.Barber{
			FullName:     form.FullName,
			PhoneNumber:  form.Phone,
			Email:        form.Email,
			PasswordHash: hash,
			ShopName:     form.ShopName,
			ShopAddress:  form.ShopAddress,
			State:        form.State,
			LocalArea:    form.LocalArea,
			Status:       models.BarberActive,
		})

		if errors.Is(err, storage.ErrEmailTaken) {
			renderBarberForm(w, s, form, 0, "That email is already registered.")

			return
		}

		if err != nil {
			serverError(w, "create barber", err)

			return
		}

		http.Redirect(w, r, "/admin/barbers", http.StatusSeeOther)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}
