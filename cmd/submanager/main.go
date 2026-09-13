// Command submanager runs the Sub-Manager server: fetch airport subscriptions,
// synthesize composed profiles via subconverter, and serve them publicly.
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/akatsukisky/sub-manager-go/internal/domain"
	"github.com/akatsukisky/sub-manager-go/internal/scheduler"
	"github.com/akatsukisky/sub-manager-go/internal/service"
	"github.com/akatsukisky/sub-manager-go/internal/web"
)

func main() {
	dbPath := getenv("SUBMANAGER_DB_PATH", "data/submanager.db")
	port := getenvInt("PORT", 8080)
	demoEnabled := getenvBool("SUBMANAGER_DEMO_ENABLED", true)

	if dir := filepath.Dir(dbPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			slog.Error("failed to create data directory", "dir", dir, "err", err)
			os.Exit(1)
		}
	}

	db, err := domain.Open(dbPath)
	if err != nil {
		slog.Error("failed to open database", "path", dbPath, "err", err)
		os.Exit(1)
	}
	defer db.Close()

	airportRepo := domain.NewAirportRepo(db)
	profileRepo := domain.NewProfileRepo(db)
	settingsRepo := domain.NewSettingsRepo(db)

	settingsService := service.NewSettingsService(settingsRepo)
	fetcher := service.NewAirportFetcher(airportRepo, settingsService)
	subconverterClient := service.NewSubconverterClient(settingsService)
	synthesizer := service.NewProfileSynthesizer(profileRepo, airportRepo, settingsService, subconverterClient)
	orchestrator := service.NewRefreshOrchestrator(airportRepo, profileRepo, fetcher, synthesizer)

	sched := scheduler.New(orchestrator, settingsService)
	sched.Start()
	defer sched.Stop()

	deps := &web.Deps{
		Airports:     airportRepo,
		Profiles:     profileRepo,
		Settings:     settingsService,
		Fetcher:      fetcher,
		Synthesizer:  synthesizer,
		Orchestrator: orchestrator,
		ServerPort:   port,
		DemoEnabled:  demoEnabled,
	}
	handler := web.NewRouter(deps)

	addr := fmt.Sprintf(":%d", port)
	slog.Info("starting sub-manager-go", "addr", addr, "dbPath", dbPath, "demoEnabled", demoEnabled)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getenvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
