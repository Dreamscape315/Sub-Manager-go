package service

import (
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/akatsukisky/sub-manager-go/internal/domain"
)

// ProfileSynthesizer synthesizes one Profile's cached_content by calling
// subconverter with a "|"-joined list of this platform's own internal
// endpoints (not the airports' raw URLs) - see buildInternalRawURL. That way
// a temporarily-dead airport doesn't break synthesis as long as it was
// fetched successfully at least once before.
type ProfileSynthesizer struct {
	profiles     *domain.ProfileRepo
	airports     *domain.AirportRepo
	settings     *SettingsService
	subconverter *SubconverterClient
}

func NewProfileSynthesizer(profiles *domain.ProfileRepo, airports *domain.AirportRepo,
	settings *SettingsService, subconverter *SubconverterClient) *ProfileSynthesizer {
	return &ProfileSynthesizer{profiles: profiles, airports: airports, settings: settings, subconverter: subconverter}
}

// Synthesize regenerates the given profile's cached content. Returns true if
// cachedContent was updated; false covers disabled/missing/no-usable-airport/call-failed,
// all of which intentionally leave the previous cachedContent in place (degrade).
func (s *ProfileSynthesizer) Synthesize(profileID int64) bool {
	profile, err := s.profiles.FindByID(profileID)
	if err != nil || profile == nil {
		slog.Warn("profile not found, skip", "id", profileID)
		return false
	}
	if !profile.Enabled {
		return false
	}

	allEnabled, err := s.airports.FindEnabledOrderByID()
	if err != nil {
		slog.Warn("failed to load airports for synthesis", "profileId", profileID, "err", err)
		return false
	}
	var usable []domain.Airport
	for _, a := range allEnabled {
		if a.HasCachedContent() && profile.IncludesAirport(a.ID) {
			usable = append(usable, a)
		}
	}
	if len(usable) == 0 {
		slog.Warn("profile has no airport with cached content; keeping old cachedContent",
			"profileId", profileID, "slug", profile.Slug)
		return false
	}

	settings := s.settings.Get()
	token := settings.InternalSecret
	urls := make([]string, 0, len(usable))
	for _, a := range usable {
		urls = append(urls, buildInternalRawURL(settings.InternalBaseURL, a.ID, token))
	}
	urlList := strings.Join(urls, "|")

	slog.Info("calling subconverter", "profileId", profileID, "slug", profile.Slug,
		"target", profile.TargetFormat, "config", orNone(profile.ExternalConfig),
		"airports", len(usable), "tokenized", token != "")

	result, err := s.subconverter.Convert(profile.TargetFormat, urlList, profile.ExternalConfig)
	if err != nil {
		slog.Warn("profile synthesize failed (old cachedContent kept)",
			"profileId", profileID, "slug", profile.Slug, "err", err)
		return false
	}
	if err := s.profiles.UpdateCachedContent(profileID, result.Body, result.ContentType, time.Now()); err != nil {
		slog.Error("failed to persist synthesized content", "profileId", profileID, "err", err)
		return false
	}
	slog.Info("profile synthesized", "profileId", profileID, "slug", profile.Slug,
		"airports", len(usable), "bytes", len(result.Body), "contentType", result.ContentType)
	return true
}

// buildInternalRawURL builds "{base}/internal/airports/{id}/raw[?token=xxx]".
func buildInternalRawURL(base string, airportID int64, token string) string {
	trimmed := strings.TrimRight(base, "/")
	u := trimmed + "/internal/airports/" + strconv.FormatInt(airportID, 10) + "/raw"
	if strings.TrimSpace(token) != "" {
		u += "?token=" + url.QueryEscape(token)
	}
	return u
}

func orNone(s string) string {
	if s == "" {
		return "<none>"
	}
	return s
}
