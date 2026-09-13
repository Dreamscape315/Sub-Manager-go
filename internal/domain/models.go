// Package domain holds the persisted entities and their SQLite-backed repositories.
package domain

import "time"

// Airport is a subscription provider ("机场"). Protocol parsing/rendering is
// entirely delegated to subconverter; this struct only carries cached data.
type Airport struct {
	ID               int64
	Name             string
	SubURL           string
	SourceType       string // "URL" or "MANUAL"
	ManualContent    string
	Enabled          bool
	CachedRawContent string
	CachedUserinfo   string
	LastFetchedAt    time.Time // zero value = never fetched
	LastFetchOK      bool
	LastFetchError   string
}

func (a *Airport) IsManualSource() bool { return a.SourceType == "MANUAL" }

func (a *Airport) HasCachedContent() bool { return a.CachedRawContent != "" }

// Profile is an outward-facing composed subscription.
type Profile struct {
	ID                 int64
	Slug               string
	TargetFormat       string
	ExternalConfig     string
	Enabled            bool
	CachedContent      string
	CachedContentType  string
	LastGeneratedAt    time.Time // zero value = never synthesized
	SelectedAirportIDs []int64   // empty = all enabled airports participate
}

// SlugPattern mirrors the Java entity's validation regex.
const SlugPattern = `^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$`

func (p *Profile) HasCachedContent() bool { return p.CachedContent != "" }

// IncludesAirport reports whether the given airport participates in this profile.
func (p *Profile) IncludesAirport(airportID int64) bool {
	if len(p.SelectedAirportIDs) == 0 {
		return true
	}
	for _, id := range p.SelectedAirportIDs {
		if id == airportID {
			return true
		}
	}
	return false
}

// AppSettings is the single global config row (id is always 1).
type AppSettings struct {
	ID                          int64
	RefreshIntervalSeconds      int
	SubconverterTimeoutSeconds  int
	AirportFetchTimeoutSeconds  int
	SubconverterBaseURL         string
	InternalBaseURL             string
	PublicBaseURL               string
	InternalSecret              string
}

const SettingsSingletonID = int64(1)

// DefaultSettings mirrors the field defaults declared on the Java entity.
func DefaultSettings() AppSettings {
	return AppSettings{
		ID:                         SettingsSingletonID,
		RefreshIntervalSeconds:     3600,
		SubconverterTimeoutSeconds: 30,
		AirportFetchTimeoutSeconds: 20,
		SubconverterBaseURL:        "http://subconverter:25500",
		InternalBaseURL:            "http://app:8080",
	}
}
