package controllerinput

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/platform"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const pollInterval = 16 * time.Millisecond

type stateReader interface {
	Snapshots() []snapshot
}

type Service struct {
	mu      sync.Mutex
	baseCtx context.Context
	cancel  context.CancelFunc
	enabled bool
	state   pollState
	reader  stateReader
	clock   func() time.Time
	emit    func(Action)
}

func NewService() *Service {
	return &Service{
		state: newPollState(),
		clock: time.Now,
		emit:  emitAction,
	}
}

func (s *Service) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	s.mu.Lock()
	s.baseCtx = ctx
	s.reader = newStateReader()
	s.mu.Unlock()

	settings, err := loadCurrentSettings()
	if err != nil {
		controllerLog().Warn("controller input startup settings unavailable", slog.Any("err", err))
		return nil
	}
	s.SetEnabled(settings.ControllerSupportEnabled)
	return nil
}

func (s *Service) ServiceShutdown() error {
	s.stop()
	return nil
}

func (s *Service) SetEnabled(enabled bool) {
	s.mu.Lock()
	s.enabled = enabled
	baseCtx := s.baseCtx
	hasLoop := s.cancel != nil
	reader := s.reader
	if !enabled {
		s.state = newPollState()
		cancel := s.cancel
		s.cancel = nil
		s.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		return
	}
	if hasLoop || baseCtx == nil || reader == nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(baseCtx)
	s.cancel = cancel
	s.state = newPollState()
	s.mu.Unlock()

	go s.run(ctx, reader)
}

func (s *Service) run(ctx context.Context, reader stateReader) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	s.pollOnce(reader)

	for {
		select {
		case <-ctx.Done():
			s.clearCancel()
			return
		case <-ticker.C:
			s.pollOnce(reader)
		}
	}
}

func (s *Service) pollOnce(reader stateReader) {
	s.mu.Lock()
	state := s.state
	now := s.clock()
	s.mu.Unlock()

	nextState, actions := advancePollState(state, reader.Snapshots(), now)

	s.mu.Lock()
	s.state = nextState
	enabled := s.enabled
	emit := s.emit
	s.mu.Unlock()

	if !enabled {
		return
	}
	for _, action := range actions {
		emit(action)
	}
}

func (s *Service) stop() {
	s.mu.Lock()
	s.enabled = false
	s.state = newPollState()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Service) clearCancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.enabled {
		s.cancel = nil
	}
}

func controllerLog() *slog.Logger {
	return slog.Default().With("component", "controller-input")
}

func emitAction(action Action) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(EventName, string(action))
}

func loadCurrentSettings() (platform.AppSettings, error) {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return platform.AppSettings{}, err
	}
	return platform.LoadAppSettings(exeDir)
}
