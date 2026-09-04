package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log"
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

		tmpl, err := template.New(entry.Name()).ParseFiles(path)
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
		http.Error(w, "Template Error", http.StatusInternalServerError)

		return
	}

	var buf bytes.Buffer

	if err := tmpl.Execute(&buf, data); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "Template Execution Error", http.StatusInternalServerError)

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
	http.Error(w, "Something went wrong. Please try again.", http.StatusInternalServerError)
}

// methodNotAllowed replies with 405 and advertises what is accepted.
func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
}
