package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

func AdminLoginHandler(w http.ResponseWriter, r *http.Request) {

	switch r.Method {

	case http.MethodGet:

		tmpl, err := template.ParseFiles("templates/admin/login.html")
		if err != nil {
			http.Error(w, "Template Error", http.StatusInternalServerError)
			return
		}

		tmpl.Execute(w, nil)

	case http.MethodPost:

		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")

		if username == "" {
			fmt.Fprintln(w, "Username is required.")
			return
		}

		if password == "" {
			fmt.Fprintln(w, "Password is required.")
			return
		}

		if username != "admin" || password != "password123" {
			fmt.Fprintln(w, "Invalid username or password.")
			return
		}

		http.Redirect(w, r, "/admin/barbers", http.StatusSeeOther)

	default:

		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)

	}
}