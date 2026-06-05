package http

import (
	"html/template"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
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
		"add1": func(i int) int { return i + 1 },
		"formatTime":    func(t time.Time) string { return t.Format("Jan 2, 2006") },
		"formatTimePtr": func(t *time.Time) string {
			if t == nil {
				return ""
			}
			return t.Format("Jan 2, 2006")
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

func noCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

func render(w http.ResponseWriter, page string, data any) {
	t, ok := pages[page]
	if !ok {
		http.Error(w, "template not found: "+page, http.StatusInternalServerError)
		return
	}
	noCache(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderPartial(w http.ResponseWriter, name string, data any) {
	noCache(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := partials.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
