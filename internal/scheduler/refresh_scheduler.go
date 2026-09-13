// Package scheduler implements the dynamically-rescheduling refresh timer.
package scheduler

import (
	"log/slog"
	"sync"
	"time"

	"github.com/akatsukisky/sub-manager-go/internal/domain"
	"github.com/akatsukisky/sub-manager-go/internal/service"
)

const startupDelay = 5 * time.Second

// RefreshScheduler runs RefreshOrchestrator.RefreshAll on a timer whose period
// is AppSettings.RefreshIntervalSeconds. Unlike a plain ticker, the period can
// change at runtime (via the settings-changed channel) and takes effect
// immediately instead of waiting for the current interval to elapse - mirroring
// the Java version's rationale for not using a fixed cron Trigger.
type RefreshScheduler struct {
	orchestrator *service.RefreshOrchestrator
	settings     *service.SettingsService
	changes      <-chan domain.AppSettings

	mu                 sync.Mutex
	timer              *time.Timer
	currentInterval    time.Duration
	lastScheduledStart time.Time
	stopCh             chan struct{}
	running            bool
}

func New(orchestrator *service.RefreshOrchestrator, settings *service.SettingsService) *RefreshScheduler {
	return &RefreshScheduler{
		orchestrator: orchestrator,
		settings:     settings,
		changes:      settings.Subscribe(),
		stopCh:       make(chan struct{}),
	}
}

// Start begins the scheduler: first run ~5s after startup, then every
// interval seconds (fixed-delay: the period is measured from when the
// previous run *started*, matching scheduleWithFixedDelay in the Java version).
func (s *RefreshScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	firstRun := time.Now().Add(startupDelay)
	s.scheduleFromLocked(firstRun)
	s.mu.Unlock()

	slog.Info("refresh scheduler started", "firstRunAround", firstRun, "intervalSeconds", s.currentInterval.Seconds())
	go s.loop()
}

func (s *RefreshScheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	if s.timer != nil {
		s.timer.Stop()
	}
	s.mu.Unlock()
	close(s.stopCh)
	slog.Info("refresh scheduler stopped")
}

func (s *RefreshScheduler) loop() {
	for {
		s.mu.Lock()
		timerC := s.timer.C
		s.mu.Unlock()

		select {
		case <-timerC:
			s.runOnce()
		case newSettings := <-s.changes:
			s.reschedule(newSettings.RefreshIntervalSeconds)
		case <-s.stopCh:
			return
		}
	}
}

func (s *RefreshScheduler) runOnce() {
	start := time.Now()
	s.mu.Lock()
	s.lastScheduledStart = start
	s.mu.Unlock()

	slog.Info("refresh cycle triggered", "at", start)
	func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("refresh cycle panicked", "recover", r)
			}
		}()
		s.orchestrator.RefreshAll()
	}()

	interval := time.Duration(s.settings.Get().RefreshIntervalSeconds) * time.Second
	s.mu.Lock()
	s.currentInterval = interval
	s.timer.Reset(interval)
	s.mu.Unlock()
}

// reschedule replans the timer so a changed interval takes effect immediately,
// instead of waiting for the currently pending tick to fire.
func (s *RefreshScheduler) reschedule(newIntervalSeconds int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	newInterval := time.Duration(newIntervalSeconds) * time.Second
	if newInterval == s.currentInterval {
		return
	}
	base := s.lastScheduledStart
	if base.IsZero() {
		base = time.Now()
	}
	candidate := base.Add(newInterval)
	now := time.Now()
	firstRun := candidate
	if candidate.Before(now) {
		firstRun = now
	}
	slog.Info("refresh interval changed", "oldSeconds", s.currentInterval.Seconds(),
		"newSeconds", newInterval.Seconds(), "nextRun", firstRun)
	s.scheduleFromLocked(firstRun)
}

// scheduleFromLocked (re)creates the timer to fire at firstRun. Caller must hold s.mu.
func (s *RefreshScheduler) scheduleFromLocked(firstRun time.Time) {
	if s.timer != nil {
		s.timer.Stop()
	}
	interval := time.Duration(s.settings.Get().RefreshIntervalSeconds) * time.Second
	s.currentInterval = interval
	s.lastScheduledStart = firstRun
	delay := time.Until(firstRun)
	if delay < 0 {
		delay = 0
	}
	s.timer = time.NewTimer(delay)
}
