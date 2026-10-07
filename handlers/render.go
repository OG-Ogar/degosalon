package handlers

import (
	"bytes"
	"barber-booking/internal/geography"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"math"
	"net/url"
	"reflect"
	"net/http"
	"path/filepath"
	"strings"
)

// templates holds every page parsed once at startup, keyed by its path
// relative to the templates directory, for example "barber/login.html".
//
// Parsing on every request (the previous behaviour) re-read the files
// from disk for each visitor and turned a typo in a template into a
// runtime 500 that only showed up when that page was opened.
var templates = map[string]*template.Template{}

// LoadTemplates parses every .html file under dir. It returns an error
// if any template is malformed, so the server refuses to start rather
// than failing later in front of a user.
func LoadTemplates(dir string) error {
	found := map[string]*template.Template{}
	partials, err := filepath.Glob(filepath.Join(dir, "partials", "*.html"))
	if err != nil {
		return err
	}

	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") {
			return nil
		}

		name, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		tmpl, err := template.New(entry.Name()).Funcs(template.FuncMap{"navigation": navigation, "naira": naira, "geographyStates": geography.States, "geographyStateID": geography.StateID, "geographyAreaID": geography.AreaID}).ParseFiles(append([]string{path}, partials...)...)
		if err != nil {
			return fmt.Errorf("parse %s: %w", name, err)
		}

		found[filepath.ToSlash(name)] = tmpl

		return nil
	}

	if err := filepath.WalkDir(dir, walk); err != nil {
		return err
	}

	if len(found) == 0 {
		return fmt.Errorf("no templates found under %s", dir)
	}

	templates = found

	return nil
}

// render writes a template to the response with a 200 status.
func render(w http.ResponseWriter, name string, data any) {
	renderStatus(w, http.StatusOK, name, data)
}

// renderStatus writes a template to the response with an explicit
// status code, used when re-showing a form that was rejected.
//
// The page is rendered into a buffer first. If execution fails halfway
// the client gets a clean error instead of a truncated page with a 200
// status already committed.
func renderStatus(w http.ResponseWriter, status int, name string, data any) {
	tmpl, found := templates[name]
	if !found {
		log.Printf("render: template %q is not loaded", name)
		journeyError(w, "Template Error", http.StatusInternalServerError)

		return
	}

	var buf bytes.Buffer

	if err := tmpl.Execute(&buf, data); err != nil {
		log.Printf("render %s: %v", name, err)
		journeyError(w, "Template Execution Error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if _, err := buf.WriteTo(w); err != nil {
		log.Printf("write %s: %v", name, err)
	}
}

// serverError logs the real cause and shows the visitor a generic
// message, so internal details never reach the browser.
func serverError(w http.ResponseWriter, context string, err error) {
	log.Printf("%s: %v", context, err)
	journeyError(w, "Something went wrong. Please try again.", http.StatusInternalServerError)
}

// methodNotAllowed replies with 405 and advertises what is accepted.
func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	journeyError(w, "Method Not Allowed", http.StatusMethodNotAllowed)
}

type navigationLink struct {
    Path string
    Label string
    Active bool
}
type navigationView struct {
    Role string
    CSRF string
    Links []navigationLink
}
// Optional presentation fields let every page share navigation without
// requiring its domain view model to implement a navigation interface.
func viewString(data any, name string) string {
    value := reflect.ValueOf(data)
    for value.IsValid() && value.Kind() == reflect.Pointer {
        if value.IsNil() { return "" }
        value = value.Elem()
    }
    if !value.IsValid() { return "" }
    var field reflect.Value
    if value.Kind() == reflect.Struct { field = value.FieldByName(name) }
    if value.Kind() == reflect.Map && value.Type().Key().Kind() == reflect.String {
        field = value.MapIndex(reflect.ValueOf(name).Convert(value.Type().Key()))
        if field.IsValid() && field.Kind() == reflect.Interface { field = field.Elem() }
    }
    if field.IsValid() && field.Kind() == reflect.String { return field.String() }
    return ""
}
func navigation(role, active string, data any) navigationView {
    view := navigationView{Role: role, CSRF: viewString(data, "CSRF")}
    destinations := [][2]string{{"dashboard", "Dashboard"}}
    switch role {
    case "customer":
        destinations = append(destinations, [2]string{"barbers", "Find a Barber"}, [2]string{"products", "Products"}, [2]string{"cart", "Cart"}, [2]string{"bookings", "My Bookings"}, [2]string{"chats", "Messages"}, [2]string{"profile", "My Profile"})
    case "barber":
        destinations = append(destinations, [2]string{"hairstyles", "Hairstyles"}, [2]string{"products", "Products"}, [2]string{"schedule", "Schedule"}, [2]string{"bookings", "Bookings"}, [2]string{"chats", "Messages"}, [2]string{"profile", "My Profile"})
    case "admin":
        destinations = append(destinations, [2]string{"barbers", "Barbers"}, [2]string{"customers", "Customers"}, [2]string{"bookings", "Bookings"})
    }
    for _, item := range destinations {
        path := "/" + role + "/" + item[0]
        if role == "customer" && item[0] == "barbers" {
            if results := viewString(data, "ResultsLink"); strings.HasPrefix(results, "/customer/barbers") {
                path = results
            } else if state := viewString(data, "SelectedState"); state != "" {
                query := url.Values{"state": {state}, "area": {viewString(data, "SelectedArea")}}
                path += "?" + query.Encode()
            }
        }
        view.Links = append(view.Links, navigationLink{Path: path, Label: item[1], Active: item[0] == active})
    }
    return view
}
// Presentation only: storage and checkout continue to own money validation.
func naira(amount float64) string {
    if math.IsNaN(amount) || math.IsInf(amount, 0) { return "Price unavailable" }
    text := fmt.Sprintf("%.2f", amount)
    parts := strings.SplitN(text, ".", 2)
    digits := parts[0]
    sign := ""
    if strings.HasPrefix(digits, "-") { sign = "-"; digits = strings.TrimPrefix(digits, "-") }
    for i := len(digits)-3; i > 0; i -= 3 { digits = digits[:i] + "," + digits[i:] }
    return "₦" + sign + digits + "." + parts[1]
}

var journeyErrorTemplate = template.Must(template.New("journey-error").Parse(`<!DOCTYPE html>
<html lang="en"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}} - De Go Salon</title><link rel="stylesheet" href="/static/app.css"></head>
<body class="modern-role-page"><main class="journey-error-card">
<a href="/">De Go Salon</a><p>{{.Status}}</p><h1>{{.Title}}</h1><p role="alert">{{.Message}}</p>
<p>You can return to your workspace or sign in again to continue.</p>
<nav aria-label="Recovery options"><a href="/customer/dashboard">Customer dashboard</a><a href="/barber/dashboard">Barber dashboard</a><a href="/explore">Find a Barber</a><a href="/">Home</a></nav>
</main></body></html>`))

func journeyError(w http.ResponseWriter, message string, status int) {
    var body bytes.Buffer
    data := struct { Status int; Title, Message string }{status, http.StatusText(status), message}
    if err := journeyErrorTemplate.Execute(&body, data); err != nil {
        http.Error(w, "Something went wrong. Please try again.", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.Header().Set("X-Content-Type-Options", "nosniff")
    w.Header().Set("Cache-Control", "no-store")
    w.WriteHeader(status)
    _, _ = body.WriteTo(w)
}
