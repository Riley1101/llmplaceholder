package http

import (
	"html/template"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

var (
	pages    map[string]*template.Template // per-page sets: layout + partials + page content
	partials *template.Template            // partials-only set for HTMX fragments
)

func LoadTemplates(dir string) error {
	funcs := template.FuncMap{
		"pathEscape": url.PathEscape,
		"activeClass": func(active, tab string) string {
			if active == tab {
				return "active"
			}
			return ""
		},
	}

	// base = layout + partials; cloned for each page
	base, err := template.New("").Funcs(funcs).ParseFiles(filepath.Join(dir, "layout.html"))
	if err != nil {
		return err
	}
	base, err = base.ParseGlob(filepath.Join(dir, "partials", "*.html"))
	if err != nil {
		return err
	}
	partials = base

	pageFiles, err := filepath.Glob(filepath.Join(dir, "pages", "*.html"))
	if err != nil {
		return err
	}

	pages = make(map[string]*template.Template, len(pageFiles))
	for _, f := range pageFiles {
		name := strings.TrimSuffix(filepath.Base(f), ".html")
		t, err := base.Clone()
		if err != nil {
			return err
		}
		t, err = t.ParseFiles(f)
		if err != nil {
			return err
		}
		pages[name] = t
	}
	return nil
}

func render(w http.ResponseWriter, page string, data any) {
	t, ok := pages[page]
	if !ok {
		http.Error(w, "template not found: "+page, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderPartial(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := partials.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
