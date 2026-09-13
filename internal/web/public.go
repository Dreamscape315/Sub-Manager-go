package web

import (
	"net/http"
	"strings"

	"github.com/akatsukisky/sub-manager-go/internal/service"
)

// Subscribe handles GET /profile/{slug}: the public subscription endpoint.
// slug missing / disabled / never synthesized -> 404 (never 403, so slug
// existence isn't leaked). Content-Type follows the last synthesis's declared
// type, falling back to a guess from target_format. Cache-Control: no-store
// keeps Cloudflare's edge cache from serving stale content after a refresh.
func (d *Deps) Subscribe(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	profile, err := d.Profiles.FindBySlug(slug)
	if err != nil || profile == nil || !profile.Enabled || !profile.HasCachedContent() {
		http.NotFound(w, r)
		return
	}

	contentType := profile.CachedContentType
	if strings.TrimSpace(contentType) == "" {
		contentType = mediaTypeFor(profile.TargetFormat)
	}

	airports, _ := d.Airports.FindEnabledOrderByID()
	var headers []string
	for _, a := range airports {
		if a.HasCachedContent() && profile.IncludesAirport(a.ID) {
			headers = append(headers, a.CachedUserinfo)
		}
	}
	if agg, ok := service.AggregateUserinfo(headers); ok {
		w.Header().Set("Subscription-Userinfo", agg)
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(profile.CachedContent))
}

// mediaTypeFor guesses a Content-Type from the subconverter target format,
// used only when the last synthesis didn't record one.
func mediaTypeFor(target string) string {
	switch strings.ToLower(target) {
	case "clash", "clashr":
		return "application/yaml; charset=utf-8"
	case "singbox", "sing-box":
		return "application/json; charset=utf-8"
	default:
		return "text/plain; charset=utf-8"
	}
}

// InternalAirportRaw handles GET /internal/airports/{id}/raw: subconverter's
// docker-compose-internal callback to read a cached raw subscription. All
// failure branches return 404 (not 401/403) so the endpoint's existence isn't
// exposed to a scanner; an optional ?token= guards it when InternalSecret is set.
func (d *Deps) InternalAirportRaw(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	settings := d.Settings.Get()
	if expected := settings.InternalSecret; strings.TrimSpace(expected) != "" && r.URL.Query().Get("token") != expected {
		http.NotFound(w, r)
		return
	}
	airport, err := d.Airports.FindByID(id)
	if err != nil || airport == nil || !airport.Enabled || !airport.HasCachedContent() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if airport.CachedUserinfo != "" {
		w.Header().Set("Subscription-Userinfo", airport.CachedUserinfo)
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(airport.CachedRawContent))
}

func pathInt64(r *http.Request, name string) (int64, bool) {
	return parseInt64(r.PathValue(name))
}
