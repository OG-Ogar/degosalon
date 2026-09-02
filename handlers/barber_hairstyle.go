package handlers

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"barber-booking/models"
	"barber-booking/storage"
)

func NewHairstyleHandler(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/barber/hairstyles/new" {
		http.NotFound(w, r)
		return
	}

	barber, found := CurrentBarber(r)
	if !found {
		http.Redirect(w, r, "/barber/login", http.StatusSeeOther)
		return
	}

	switch r.Method {

	case http.MethodGet:

		tmpl, err := template.ParseFiles(
			"templates/barber/hairstyle-form.html",
		)
		if err != nil {
			http.Error(w, "Template Error", http.StatusInternalServerError)
			return
		}

		if err := tmpl.Execute(w, nil); err != nil {
			http.Error(w, "Template Execution Error", http.StatusInternalServerError)
			return
		}

	case http.MethodPost:

		name := strings.TrimSpace(r.FormValue("name"))
		category := strings.TrimSpace(r.FormValue("category"))
		description := strings.TrimSpace(r.FormValue("description"))
		priceText := strings.TrimSpace(r.FormValue("price"))
		durationText := strings.TrimSpace(r.FormValue("duration"))

		if name == "" ||
			category == "" ||
			description == "" ||
			priceText == "" ||
			durationText == "" {

			http.Error(w, "All fields are required", http.StatusBadRequest)
			return
		}

		price, err := strconv.ParseFloat(priceText, 64)
		if err != nil {
			http.Error(w, "Invalid price", http.StatusBadRequest)
			return
		}

		if price < 0 {
			http.Error(
				w,
				"Price must be greater than or equal to zero",
				http.StatusBadRequest,
			)
			return
		}

		duration, err := strconv.Atoi(durationText)
		if err != nil {
			http.Error(w, "Invalid duration", http.StatusBadRequest)
			return
		}

		if duration <= 0 {
			http.Error(
				w,
				"Duration must be greater than zero",
				http.StatusBadRequest,
			)
			return
		}

		file, header, err := r.FormFile("image")
		if err != nil {
			http.Error(
				w,
				"Image upload failed: "+err.Error(),
				http.StatusBadRequest,
			)
			return
		}

		defer file.Close()

		extension := strings.ToLower(
			filepath.Ext(header.Filename),
		)

		if extension != ".jpg" &&
			extension != ".jpeg" &&
			extension != ".png" &&
			extension != ".gif" &&
			extension != ".webp" {

			http.Error(
				w,
				"Invalid image type",
				http.StatusBadRequest,
			)
			return
		}

		err = os.MkdirAll(
			"uploads/hairstyles",
			0755,
		)
		if err != nil {
			http.Error(
				w,
				"Could not create upload directory",
				http.StatusInternalServerError,
			)
			return
		}

		filename := fmt.Sprintf(
			"%d%s",
			time.Now().UnixNano(),
			extension,
		)

		filePath := filepath.Join(
			"uploads",
			"hairstyles",
			filename,
		)

		dst, err := os.Create(filePath)
		if err != nil {
			http.Error(
				w,
				"Could not create image file: "+err.Error(),
				http.StatusInternalServerError,
			)
			return
		}

		_, err = io.Copy(dst, file)
		dst.Close()

		if err != nil {
			os.Remove(filePath)

			http.Error(
				w,
				"Could not save image: "+err.Error(),
				http.StatusInternalServerError,
			)
			return
		}

		imagePath := "/uploads/hairstyles/" + filename

		nextID := 1

		for _, hairstyle := range storage.Hairstyles {

			if hairstyle.ID >= nextID {
				nextID = hairstyle.ID + 1
			}
		}

		hairstyle := models.Hairstyle{
			ID:          nextID,
			BarberID:    barber.ID,
			Name:        name,
			Category:    category,
			Description: description,
			Price:       price,
			Duration:    duration,
			Image:       imagePath,
			IsAvailable: true,
		}

		storage.Hairstyles = append(
			storage.Hairstyles,
			hairstyle,
		)

		http.Redirect(
			w,
			r,
			"/barber/hairstyles",
			http.StatusSeeOther,
		)

	default:

		http.Error(
			w,
			"Method Not Allowed",
			http.StatusMethodNotAllowed,
		)
	}
}

func ListHairstylesHandler(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/barber/hairstyles" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	barber, found := CurrentBarber(r)
	if !found {
		http.Redirect(w, r, "/barber/login", http.StatusSeeOther)
		return
	}

	var hairstyles []models.Hairstyle

	for _, hairstyle := range storage.Hairstyles {

		if hairstyle.BarberID == barber.ID {
			hairstyles = append(hairstyles, hairstyle)
		}
	}

	tmpl, err := template.ParseFiles(
		"templates/barber/hairstyle-list.html",
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, hairstyles); err != nil {
		http.Error(w, "Template Execution Error", http.StatusInternalServerError)
		return
	}
}

func ViewHairstyleHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")

	if len(parts) != 4 ||
		parts[1] != "barber" ||
		parts[2] != "hairstyles" ||
		parts[3] == "" {

		http.NotFound(w, r)
		return
	}

	id, err := strconv.Atoi(parts[3])
	if err != nil {
		http.Error(w, "Invalid Hairstyle ID", http.StatusBadRequest)
		return
	}

	barber, found := CurrentBarber(r)
	if !found {
		http.Redirect(w, r, "/barber/login", http.StatusSeeOther)
		return
	}

	hairstyle, found := storage.FindHairstyleByID(id)
	if !found {
		http.NotFound(w, r)
		return
	}

	if hairstyle.BarberID != barber.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	tmpl, err := template.ParseFiles(
		"templates/barber/hairstyle-profile.html",
	)
	if err != nil {
		http.Error(w, "Template Error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, hairstyle); err != nil {
		http.Error(w, "Template Execution Error", http.StatusInternalServerError)
		return
	}
}

func EditHairstyleHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")

	if len(parts) != 5 ||
		parts[1] != "barber" ||
		parts[2] != "hairstyles" ||
		parts[3] == "" ||
		parts[4] != "edit" {

		http.NotFound(w, r)
		return
	}

	id, err := strconv.Atoi(parts[3])
	if err != nil {
		http.Error(w, "Invalid Hairstyle ID", http.StatusBadRequest)
		return
	}

	barber, found := CurrentBarber(r)
	if !found {
		http.Redirect(w, r, "/barber/login", http.StatusSeeOther)
		return
	}

	hairstyle, found := storage.FindHairstyleByID(id)
	if !found {
		http.NotFound(w, r)
		return
	}

	if hairstyle.BarberID != barber.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	tmpl, err := template.ParseFiles(
		"templates/barber/edit-hairstyle.html",
	)
	if err != nil {
		http.Error(w, "Template Error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, hairstyle); err != nil {
		http.Error(w, "Template Execution Error", http.StatusInternalServerError)
		return
	}
}

func UpdateHairstyleHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")

	if len(parts) != 5 ||
		parts[1] != "barber" ||
		parts[2] != "hairstyles" ||
		parts[3] == "" ||
		parts[4] != "update" {

		http.NotFound(w, r)
		return
	}

	id, err := strconv.Atoi(parts[3])
	if err != nil {
		http.Error(w, "Invalid Hairstyle ID", http.StatusBadRequest)
		return
	}

	barber, found := CurrentBarber(r)
	if !found {
		http.Redirect(w, r, "/barber/login", http.StatusSeeOther)
		return
	}

	hairstyle, found := storage.FindHairstyleByID(id)
	if !found {
		http.NotFound(w, r)
		return
	}

	if hairstyle.BarberID != barber.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	category := strings.TrimSpace(r.FormValue("category"))
	description := strings.TrimSpace(r.FormValue("description"))
	priceText := strings.TrimSpace(r.FormValue("price"))
	durationText := strings.TrimSpace(r.FormValue("duration"))
	image := strings.TrimSpace(r.FormValue("image"))

	if name == "" ||
		category == "" ||
		description == "" ||
		priceText == "" ||
		durationText == "" {

		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	price, err := strconv.ParseFloat(priceText, 64)
	if err != nil {
		http.Error(w, "Invalid Price", http.StatusBadRequest)
		return
	}

	if price <= 0 {
		http.Error(w, "Price must be greater than zero", http.StatusBadRequest)
		return
	}

	duration, err := strconv.Atoi(durationText)
	if err != nil {
		http.Error(w, "Invalid Duration", http.StatusBadRequest)
		return
	}

	if duration <= 0 {
		http.Error(w, "Duration must be greater than zero", http.StatusBadRequest)
		return
	}

	hairstyle.Name = name
	hairstyle.Category = category
	hairstyle.Description = description
	hairstyle.Price = price
	hairstyle.Duration = duration
	hairstyle.Image = image
	hairstyle.IsAvailable = r.FormValue("available") == "on"

	http.Redirect(
		w,
		r,
		fmt.Sprintf("/barber/hairstyles/%d", hairstyle.ID),
		http.StatusSeeOther,
	)
}

func ConfirmDeleteHairstyleHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")

	if len(parts) != 5 ||
		parts[1] != "barber" ||
		parts[2] != "hairstyles" ||
		parts[3] == "" ||
		parts[4] != "delete" {

		http.NotFound(w, r)
		return
	}

	id, err := strconv.Atoi(parts[3])
	if err != nil {
		http.Error(w, "Invalid Hairstyle ID", http.StatusBadRequest)
		return
	}

	barber, found := CurrentBarber(r)
	if !found {
		http.Redirect(w, r, "/barber/login", http.StatusSeeOther)
		return
	}

	hairstyle, found := storage.FindHairstyleByID(id)
	if !found {
		http.NotFound(w, r)
		return
	}

	if hairstyle.BarberID != barber.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	tmpl, err := template.ParseFiles(
		"templates/barber/confirm-delete-hairstyle.html",
	)
	if err != nil {
		http.Error(w, "Template Error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, hairstyle); err != nil {
		http.Error(w, "Template Execution Error", http.StatusInternalServerError)
		return
	}
}

func DeleteHairstyleHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")

	if len(parts) != 5 ||
		parts[1] != "barber" ||
		parts[2] != "hairstyles" ||
		parts[3] == "" ||
		parts[4] != "delete" {

		http.NotFound(w, r)
		return
	}

	id, err := strconv.Atoi(parts[3])
	if err != nil {
		http.Error(w, "Invalid Hairstyle ID", http.StatusBadRequest)
		return
	}

	barber, found := CurrentBarber(r)
	if !found {
		http.Redirect(w, r, "/barber/login", http.StatusSeeOther)
		return
	}

	for i, hairstyle := range storage.Hairstyles {

		if hairstyle.ID == id && hairstyle.BarberID == barber.ID {

			storage.Hairstyles = append(
				storage.Hairstyles[:i],
				storage.Hairstyles[i+1:]...,
			)

			http.Redirect(
				w,
				r,
				"/barber/hairstyles",
				http.StatusSeeOther,
			)

			return
		}
	}

	http.NotFound(w, r)
}
