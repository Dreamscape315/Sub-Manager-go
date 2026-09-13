// Package web wires HTTP handlers, form validation and template rendering.
package web

import (
	"embed"
	"html/template"
	"log/slog"
	"net/http"
)

//go:embed templates
var templateFS embed.FS

//go:embed static
var StaticFS embed.FS

var tmpl *template.Template

func init() {
	t, err := template.New("root").Funcs(funcMap).ParseFS(templateFS,
		"templates/fragments/*.html",
		"templates/admin/*.html",
		"templates/admin/fragments/*.html",
	)
	if err != nil {
		panic("parse templates: " + err.Error())
	}
	tmpl = t
}

// PageData holds fields shared by every admin page (nav highlight, title,
// global settings mirror, and one-shot flash messages read from cookies).
type PageData struct {
	Title         string
	Nav           string
	PublicBaseURL string
	FlashOK       string
	FlashErr      string
}

func render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("template render failed", "template", name, "err", err)
	}
}

// renderFragment is used for htmx partial responses (no surrounding <html>).
func renderFragment(w http.ResponseWriter, name string, data any) {
	render(w, name, data)
}
