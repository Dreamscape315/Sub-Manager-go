package service

import (
	"log/slog"
	"sync"

	"github.com/akatsukisky/sub-manager-go/internal/domain"
)

// SettingsService is the read/write facade for the global AppSettings row.
// Save broadcasts the new value to subscribers (e.g. the refresh scheduler) so
// changes take effect immediately, without waiting for the next tick.
type SettingsService struct {
	repo *domain.SettingsRepo

	mu        sync.Mutex
	listeners []chan domain.AppSettings
}

func NewSettingsService(repo *domain.SettingsRepo) *SettingsService {
	return &SettingsService{repo: repo}
}

// Get returns the current settings, self-healing (inserting defaults) on first run.
func (s *SettingsService) Get() domain.AppSettings {
	settings, err := s.repo.Get()
	if err != nil {
		slog.Error("failed to load app settings, falling back to defaults", "err", err)
		return domain.DefaultSettings()
	}
	return settings
}

// Save persists settings (forcing the singleton id) and notifies subscribers.
func (s *SettingsService) Save(updated domain.AppSettings) (domain.AppSettings, error) {
	updated.ID = domain.SettingsSingletonID
	if err := s.repo.Save(updated); err != nil {
		return domain.AppSettings{}, err
	}
	slog.Info("app settings saved",
		"refreshIntervalSeconds", updated.RefreshIntervalSeconds,
		"subconverterTimeoutSeconds", updated.SubconverterTimeoutSeconds,
		"airportFetchTimeoutSeconds", updated.AirportFetchTimeoutSeconds)
	s.notify(updated)
	return updated, nil
}

// Subscribe registers a buffered channel that receives every future saved
// settings value. Never closed; intended for long-lived subscribers (scheduler).
func (s *SettingsService) Subscribe() <-chan domain.AppSettings {
	ch := make(chan domain.AppSettings, 1)
	s.mu.Lock()
	s.listeners = append(s.listeners, ch)
	s.mu.Unlock()
	return ch
}

func (s *SettingsService) notify(updated domain.AppSettings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ch := range s.listeners {
		select {
		case ch <- updated:
		default:
			// Listener hasn't drained the previous value yet; drop-and-replace
			// so a slow consumer never blocks the save path.
			select {
			case <-ch:
			default:
			}
			ch <- updated
		}
	}
}
