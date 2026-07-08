package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

func AdminLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		tmpl := template.Must(template.ParseFiles("templates/admin-login.html"))

		tmpl.Execute(w, nil)
		return
	}

	if r.Method == http.MethodPost {
		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")

		if username == "" {
			fmt.Fprintln(w, "username is required.")
			return
		}

		if password == "" {
			fmt.Fprintln(w, "password is required")
			return
		}
		
		if username != "admin"{
			fmt.Fprintln(w, "Invalid username or password")
			return
		}

		if password != "password123" {
			fmt.Fprintln(w, "Invalid username or password")
			return
		}

		http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
		return

		// fmt.Fprintln(w, "username", username)
		// fmt.Fprintln(w, "password", password)
		// return
	}
}
