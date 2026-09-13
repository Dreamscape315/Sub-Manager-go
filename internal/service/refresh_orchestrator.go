package service

import (
	"log/slog"
	"sync"
	"time"

	"github.com/akatsukisky/sub-manager-go/internal/domain"
)

// RefreshOrchestrator runs one full refresh cycle: fetch every enabled
// airport, then synthesize every enabled profile. Shared by the scheduler and
// the manual "refresh all" button.
//
// A mutex (TryLock) prevents the scheduled tick and a manual trigger from
// running concurrently; the loser returns RefreshResult{Executed: false}
// immediately rather than interleaving with the winner.
type RefreshOrchestrator struct {
	airports    *domain.AirportRepo
	profiles    *domain.ProfileRepo
	fetcher     *AirportFetcher
	synthesizer *ProfileSynthesizer

	mu sync.Mutex
}

func NewRefreshOrchestrator(airports *domain.AirportRepo, profiles *domain.ProfileRepo,
	fetcher *AirportFetcher, synthesizer *ProfileSynthesizer) *RefreshOrchestrator {
	return &RefreshOrchestrator{airports: airports, profiles: profiles, fetcher: fetcher, synthesizer: synthesizer}
}

type RefreshResult struct {
	Executed      bool
	TotalAirports int
	OKAirports    int
	TotalProfiles int
	OKProfiles    int
	Duration      time.Duration
}

func (o *RefreshOrchestrator) RefreshAll() RefreshResult {
	if !o.mu.TryLock() {
		slog.Info("another refresh cycle is in progress, skip this trigger")
		return RefreshResult{Executed: false}
	}
	defer o.mu.Unlock()
	return o.doRefresh()
}

func (o *RefreshOrchestrator) doRefresh() RefreshResult {
	started := time.Now()

	airports, err := o.airports.FindEnabledOrderByID()
	if err != nil {
		slog.Error("failed to load airports for refresh", "err", err)
		airports = nil
	}
	slog.Info("phase 1/2: fetching airports", "count", len(airports))
	airportOK := 0
	for _, a := range airports {
		if o.fetcher.Fetch(a.ID) {
			airportOK++
		}
	}
	slog.Info("phase 1/2 done", "ok", airportOK, "total", len(airports))

	profiles, err := o.profiles.FindEnabledOrderByID()
	if err != nil {
		slog.Error("failed to load profiles for refresh", "err", err)
		profiles = nil
	}
	slog.Info("phase 2/2: synthesizing profiles", "count", len(profiles))
	profileOK := 0
	for _, p := range profiles {
		if o.synthesizer.Synthesize(p.ID) {
			profileOK++
		}
	}
	slog.Info("phase 2/2 done", "ok", profileOK, "total", len(profiles))

	total := time.Since(started)
	slog.Info("refresh cycle done", "airportsOk", airportOK, "airportsTotal", len(airports),
		"profilesOk", profileOK, "profilesTotal", len(profiles), "costSeconds", total.Seconds())

	return RefreshResult{
		Executed:      true,
		TotalAirports: len(airports),
		OKAirports:    airportOK,
		TotalProfiles: len(profiles),
		OKProfiles:    profileOK,
		Duration:      total,
	}
}
