package web

import (
	"io/fs"
	"net/http"
)

// NewRouter registers every route on a Go 1.22+ method+pattern ServeMux.
func NewRouter(d *Deps) http.Handler {
	mux := http.NewServeMux()

	// Public / internal / demo endpoints.
	mux.HandleFunc("GET /profile/{slug}", d.Subscribe)
	mux.HandleFunc("GET /internal/airports/{id}/raw", d.InternalAirportRaw)
	mux.HandleFunc("GET /demo/airport/{name}", d.DemoAirport)

	// Admin: dashboard.
	mux.HandleFunc("GET /{$}", d.Root)
	mux.HandleFunc("GET /admin", d.Dashboard)
	mux.HandleFunc("GET /admin/", d.Dashboard)
	mux.HandleFunc("POST /admin/refresh", d.RefreshAll)

	// Admin: airports.
	mux.HandleFunc("GET /admin/airports", d.AirportsList)
	mux.HandleFunc("POST /admin/airports", d.AirportsCreate)
	mux.HandleFunc("POST /admin/airports/{id}", d.AirportsUpdate)
	mux.HandleFunc("POST /admin/airports/{id}/delete", d.AirportsDelete)
	mux.HandleFunc("POST /admin/airports/{id}/toggle", d.AirportsToggle)
	mux.HandleFunc("POST /admin/airports/{id}/fetch", d.AirportsFetchNow)

	// Admin: profiles.
	mux.HandleFunc("GET /admin/profiles", d.ProfilesList)
	mux.HandleFunc("POST /admin/profiles", d.ProfilesCreate)
	mux.HandleFunc("POST /admin/profiles/{id}", d.ProfilesUpdate)
	mux.HandleFunc("POST /admin/profiles/{id}/delete", d.ProfilesDelete)
	mux.HandleFunc("POST /admin/profiles/{id}/toggle", d.ProfilesToggle)
	mux.HandleFunc("POST /admin/profiles/{id}/synthesize", d.ProfilesSynthesizeNow)
	mux.HandleFunc("GET /admin/profiles/{id}/preview", d.ProfilesPreview)

	// Admin: settings.
	mux.HandleFunc("GET /admin/settings", d.SettingsView)
	mux.HandleFunc("POST /admin/settings", d.SettingsSave)
	mux.HandleFunc("POST /admin/settings/test-connectivity", d.SettingsTestConnectivity)

	// Static assets (embedded at build time; see StaticFS in render.go).
	staticRoot, err := fs.Sub(StaticFS, "static")
	if err != nil {
		panic("static fs sub: " + err.Error())
	}
	fileServer := http.FileServerFS(staticRoot)
	mux.Handle("GET /css/", cacheControl(fileServer, "public, max-age=3600"))
	mux.Handle("GET /favicon.svg", cacheControl(fileServer, "public, max-age=3600"))

	return mux
}

func cacheControl(next http.Handler, value string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", value)
		next.ServeHTTP(w, r)
	})
}
