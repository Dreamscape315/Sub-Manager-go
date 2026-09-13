package service

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/akatsukisky/sub-manager-go/internal/domain"
)

const maxFetchErrorLength = 500

// defaultUserAgent mimics Clash Meta for Android; most airports gate their
// subscription format (or anti-bot pages) on User-Agent.
const defaultUserAgent = "ClashMetaForAndroid/2.11.4.Meta (Prefer ClashMeta Format)"

// AirportFetcher fetches a single airport's raw subscription and persists it
// to Airport.CachedRawContent. Failures are swallowed and recorded as
// last_fetch_error; the old cached content is left untouched (degrade, don't clear).
type AirportFetcher struct {
	repo     *domain.AirportRepo
	settings *SettingsService
}

func NewAirportFetcher(repo *domain.AirportRepo, settings *SettingsService) *AirportFetcher {
	return &AirportFetcher{repo: repo, settings: settings}
}

// Fetch fetches the given airport. Returns true if the cached content was refreshed.
func (f *AirportFetcher) Fetch(airportID int64) bool {
	airport, err := f.repo.FindByID(airportID)
	if err != nil || airport == nil {
		slog.Warn("airport not found, skip", "id", airportID)
		return false
	}
	now := time.Now()

	if airport.IsManualSource() {
		if strings.TrimSpace(airport.ManualContent) == "" {
			return f.recordFailure(airportID, airport.Name, now, "manual content is empty")
		}
		if err := f.repo.MarkFetchSuccess(airportID, airport.ManualContent, "", now); err != nil {
			slog.Error("failed to persist manual fetch success", "id", airportID, "err", err)
			return false
		}
		slog.Info("airport manual content applied", "id", airportID, "name", airport.Name,
			"bytes", len(airport.ManualContent))
		return true
	}

	settings := f.settings.Get()
	client := &http.Client{Timeout: time.Duration(settings.AirportFetchTimeoutSeconds) * time.Second}

	req, err := http.NewRequest(http.MethodGet, airport.SubURL, nil)
	if err != nil {
		return f.recordFailure(airportID, airport.Name, now, err.Error())
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		return f.recordFailure(airportID, airport.Name, now, err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return f.recordFailure(airportID, airport.Name, now, fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return f.recordFailure(airportID, airport.Name, now, err.Error())
	}
	body := string(bodyBytes)
	if body == "" {
		return f.recordFailure(airportID, airport.Name, now, "empty response body")
	}
	if reason := classifyBody(body); reason != "" {
		return f.recordFailure(airportID, airport.Name, now,
			reason+" (preview: "+previewOf(body)+")")
	}
	userinfo := resp.Header.Get("Subscription-Userinfo")
	if err := f.repo.MarkFetchSuccess(airportID, body, userinfo, now); err != nil {
		slog.Error("failed to persist fetch success", "id", airportID, "err", err)
		return false
	}
	slog.Info("airport fetched OK", "id", airportID, "name", airport.Name,
		"bytes", len(body), "hasUserinfo", userinfo != "")
	return true
}

// classifyBody guesses whether a response body is clearly not a subscription
// (an HTML login/anti-bot page, or a JSON error payload). Empty string = looks fine.
func classifyBody(body string) string {
	trimmed := strings.TrimLeft(body, " \t\r\n")
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "<!doctype html") || strings.HasPrefix(lower, "<html") {
		return "response looks like an HTML page (likely a login/anti-bot page, not a subscription)"
	}
	if strings.HasPrefix(trimmed, "{") && (strings.Contains(lower, `"code"`) || strings.Contains(lower, `"error"`)) {
		return "response looks like a JSON error payload"
	}
	return ""
}

var whitespaceRun = regexp.MustCompile(`\s+`)

func previewOf(body string) string {
	oneLine := strings.TrimSpace(whitespaceRun.ReplaceAllString(body, " "))
	if len(oneLine) <= 120 {
		return oneLine
	}
	return oneLine[:120] + "…"
}

func (f *AirportFetcher) recordFailure(id int64, name string, at time.Time, rawError string) bool {
	msg := truncateError(rawError)
	if err := f.repo.MarkFetchFailure(id, at, msg); err != nil {
		slog.Error("failed to persist fetch failure", "id", id, "err", err)
	}
	slog.Warn("airport fetch failed", "id", id, "name", name, "err", msg)
	return false
}

func truncateError(s string) string {
	if strings.TrimSpace(s) == "" {
		return "unknown error"
	}
	if len(s) <= maxFetchErrorLength {
		return s
	}
	return s[:maxFetchErrorLength]
}
