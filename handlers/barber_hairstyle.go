package handlers

import (
	"errors"
	"math"
	"net/http"
	"strconv"

	"barber-booking/models"
	"barber-booking/storage"
)

// maxDurationMinutes is a full day, an upper bound that catches typos.
const maxDurationMinutes = 24 * 60

// hairstyleForm is the validated content of the add/edit hairstyle form.
type hairstyleForm struct {
	Name        string
	Category    string
	Description string
	Price       float64
	Duration    int
	Available   bool

	values map[string]string
}

// readHairstyleForm parses and validates the submitted fields, returning
// a message when something is wrong.
func readHairstyleForm(r *http.Request) (hairstyleForm, string) {
	priceText := trimmedField(r.PostFormValue("price"))
	durationText := trimmedField(r.PostFormValue("duration"))

	form := hairstyleForm{
		Name:        trimmedField(r.PostFormValue("name")),
		Category:    trimmedField(r.PostFormValue("category")),
		Description: truncateText(r.PostFormValue("description")),
		Available:   r.PostFormValue("available") == "on",
	}

	form.values = map[string]string{
		"name":        form.Name,
		"category":    form.Category,
		"description": form.Description,
		"price":       priceText,
		"duration":    durationText,
	}

	if form.Name == "" ||
		form.Category == "" ||
		form.Description == "" ||
		priceText == "" ||
		durationText == "" {

		return form, "Every field except the image and video is required."
	}

	price, err := strconv.ParseFloat(priceText, 64)

	// ParseFloat happily accepts "NaN" and "Inf", so check the value too.
	if err != nil || math.IsNaN(price) || math.IsInf(price, 0) || price < 0 {
		return form, "Enter a price of zero or more."
	}

	form.Price = price

	duration, err := strconv.Atoi(durationText)
	if err != nil || duration <= 0 || duration > maxDurationMinutes {
		return form, "Enter a duration in whole minutes, between 1 and 1440."
	}

	form.Duration = duration

	return form, ""
}

// hairstyleFormView backs both the add and the edit hairstyle form.
type hairstyleFormView struct {
	Editing   bool
	ID        int
	Image     string
	Available bool

	Values map[string]string
	CSRF   string
	Error  string
}

// renderHairstyleForm shows the add or edit form.
func renderHairstyleForm(
	w http.ResponseWriter,
	s sessionData,
	form hairstyleForm,
	id int,
	image string,
	problem string,
) {
	name := "barber/hairstyle-form.html"
	if id > 0 {
		name = "barber/edit-hairstyle.html"
	}

	status := http.StatusOK
	if problem != "" {
		status = http.StatusBadRequest
	}

	values := form.values
	if values == nil {
		values = map[string]string{}
	}

	renderStatus(w, status, name, hairstyleFormView{
		Editing:   id > 0,
		ID:        id,
		Image:     image,
		Available: form.Available,
		Values:    values,
		CSRF:      s.CSRFToken,
		Error:     problem,
	})
}

// NewHairstyleHandler adds a hairstyle to the signed-in barber's list.
func NewHairstyleHandler(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
) {
	switch r.Method {

	case http.MethodGet:
		renderHairstyleForm(w, s, hairstyleForm{Available: true}, 0, "", "")

	case http.MethodPost:
		// A hairstyle can contain one image up to 5 MiB and
		// one video up to 50 MiB, plus form data.
		limitFormBody(
			w,
			r,
			maxUploadBytes+maxVideoBytes+maxFormMemory,
		)

		if !requireCSRF(w, r, s) {
			return
		}

		form, problem := readHairstyleForm(r)
		if problem != "" {
			renderHairstyleForm(w, s, form, 0, "", problem)

			return
		}

		image, err := hairstyleImages.save(r, "image")

		if err != nil && !errors.Is(err, errNoUpload) {
			renderHairstyleForm(w, s, form, 0, "", err.Error())

			return
		}

		video, err := hairstyleVideos.save(r, "video")

		if err != nil && !errors.Is(err, errNoUpload) {
			removeImageIgnoringError(image)

			renderHairstyleForm(w, s, form, 0, "", err.Error())

			return
		}

		_, err = storage.CreateHairstyleContext(r.Context(), models.Hairstyle{
			BarberID:    barber.ID,
			Name:        form.Name,
			Category:    form.Category,
			Description: form.Description,
			Price:       form.Price,
			Duration:    form.Duration,
			Image:       image,
			Video:       video,
			IsAvailable: form.Available,
		})

		if err != nil {
			removeImageIgnoringError(image)
			removeVideoIgnoringError(video)

			serverError(w, "create hairstyle", err)

			return
		}

		http.Redirect(w, r, "/barber/hairstyles", http.StatusSeeOther)

	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}
