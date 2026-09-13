package web

import (
	"github.com/akatsukisky/sub-manager-go/internal/domain"
	"github.com/akatsukisky/sub-manager-go/internal/service"
)

// Deps bundles every repository/service handlers need. Passed by reference
// to each handler group instead of Spring's constructor-injection.
type Deps struct {
	Airports     *domain.AirportRepo
	Profiles     *domain.ProfileRepo
	Settings     *service.SettingsService
	Fetcher      *service.AirportFetcher
	Synthesizer  *service.ProfileSynthesizer
	Orchestrator *service.RefreshOrchestrator
	ServerPort   int
	DemoEnabled  bool
}

func (d *Deps) publicBaseURL() string { return d.Settings.Get().PublicBaseURL }
