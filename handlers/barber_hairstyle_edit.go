package handlers

import (
	"errors"
	"net/http"

	"barber-booking/models"
	"barber-booking/storage"
)

// formFromHairstyle pre-fills the edit form with the stored values.
func formFromHairstyle(hairstyle models.Hairstyle) hairstyleForm {
	form := hairstyleForm{
		Name:        hairstyle.Name,
		Category:    hairstyle.Category,
		Description: hairstyle.Description,
		Price:       hairstyle.Price,
		Duration:    hairstyle.Duration,
		Available:   hairstyle.IsAvailable,
	}

	form.values = map[string]string{
		"name":        hairstyle.Name,
		"category":    hairstyle.Category,
		"description": hairstyle.Description,
		"price":       formatPrice(hairstyle.Price),
		"duration":    formatInt(hairstyle.Duration),
	}

	return form
}

// EditHairstyleHandler shows the edit form for one hairstyle.
func EditHairstyleHandler(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
	idSegment string,
) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)

		return
	}

	hairstyle, ok := ownedHairstyle(w, r, barber, idSegment)
	if !ok {
		return
	}

	renderHairstyleForm(
		w,
		s,
		formFromHairstyle(hairstyle),
		hairstyle.ID,
		hairstyle.Image,
		hairstyle.Video,
		"",
	)
}

// UpdateHairstyleHandler saves an edit.
//
// The image is a file upload, not a text field. Previously the form
// posted the image path as text, which meant a barber could store any
// path they liked in that field, and the admin delete flow later handed
// it to os.Remove: a crafted value deleted files anywhere on disk.
//
// A request with no new file keeps the stored image. A request with one
// replaces it, and the old file is unlinked afterwards.
func UpdateHairstyleHandler(
	w http.ResponseWriter,
	r *http.Request,
	barber models.Barber,
	s sessionData,
	idSegment string,
) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)

		return
	}

	extendUploadDeadline(w)
	limitFormBody(
		w,
		r,
		maxUploadBytes+maxVideoBytes+maxFormMemory,
	)

	hairstyle, ok := ownedHairstyle(w, r, barber, idSegment)
	if !ok {
		return
	}

	if !requireCSRF(w, r, s) {
		return
	}

	form, problem := readHairstyleForm(r)
	if problem != "" {
		renderHairstyleForm(w, s, form, hairstyle.ID, hairstyle.Image, hairstyle.Video, problem)

		return
	}

	previous := hairstyle.Image
	previousVideo := hairstyle.Video

	image, err := hairstyleImages.save(r, "image")

	switch {
	case errors.Is(err, errNoUpload):
		image = previous

	case err != nil:
		renderHairstyleForm(w, s, form, hairstyle.ID, hairstyle.Image, hairstyle.Video, err.Error())

		return
	}

	video, err := hairstyleVideos.save(r, "video")

	switch {
	case errors.Is(err, errNoUpload):
		video = previousVideo

	case err != nil:
		// Only a newly uploaded photo is unreferenced at this point; the
		// stored one must survive the rejected request.
		if image != previous {
			removeImageIgnoringError(image)
		}

		renderHairstyleForm(w, s, form, hairstyle.ID, hairstyle.Image, hairstyle.Video, err.Error())

		return
	}

	// Whichever way the write goes, exactly one of these two files ends
	// up unreferenced and has to be unlinked.
	uploaded := ""

	if image != previous {
		uploaded = image
	}

	uploadedVideo := ""

	if video != previousVideo {
		uploadedVideo = video
	}

	hairstyle.Name = form.Name
	hairstyle.Category = form.Category
	hairstyle.Description = form.Description
	hairstyle.Price = form.Price
	hairstyle.Duration = form.Duration
	hairstyle.IsAvailable = form.Available
	hairstyle.Image = image
	hairstyle.Video = video

	if err := storage.UpdateHairstyleContext(r.Context(), hairstyle); err != nil {
		removeImageIgnoringError(uploaded)
		removeVideoIgnoringError(uploadedVideo)
		serverError(w, "update hairstyle", err)

		return
	}

	if uploaded != "" {
		removeImageIgnoringError(previous)
	}

	if uploadedVideo != "" {
		removeVideoIgnoringError(previousVideo)
	}

	http.Redirect(w, r, "/barber/hairstyles", http.StatusSeeOther)
}
