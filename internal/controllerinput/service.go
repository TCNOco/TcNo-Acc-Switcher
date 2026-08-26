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
	visible bool
	focused bool
	state   pollState
	reader  stateReader
	clock   func() time.Time
	emit    func(Action)
}

func NewService() *Service {
	return &Service{
		state: newPollState(),
		// Both start true so a caller that never reports window state polls
		// exactly as it did before.
		visible: true,
		focused: true,
		clock:   time.Now,
		emit:    emitAction,
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
	s.mu.Unlock()
	s.syncLoop()
}

// SetWindowVisible suspends polling while the window is hidden or minimised.
// Controller input only ever drives that window's UI, so a window sitting in the
// tray would be polling XInput for nobody.
func (s *Service) SetWindowVisible(visible bool) {
	s.mu.Lock()
	if s.visible == visible {
		s.mu.Unlock()
		return
	}
	s.visible = visible
	s.mu.Unlock()
	s.syncLoop()
}

// SetWindowFocused suspends polling while another application is in front.
// XInput has no notion of focus, so without this a stick pushed in a game or a
// browser would still be walking the account list behind it.
func (s *Service) SetWindowFocused(focused bool) {
	s.mu.Lock()
	if s.focused == focused {
		s.mu.Unlock()
		return
	}
	s.focused = focused
	s.mu.Unlock()
	s.syncLoop()
}

// shouldPollLocked reports whether anyone can act on what the poll reads. The
// caller holds s.mu.
func (s *Service) shouldPollLocked() bool {
	return s.enabled && s.visible && s.focused
}

// syncLoop brings the poll goroutine in line with shouldPollLocked.
func (s *Service) syncLoop() {
	s.mu.Lock()
	if !s.shouldPollLocked() {
		s.state = newPollState()
		cancel := s.cancel
		s.cancel = nil
		s.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		return
	}
	reader := s.reader
	if s.cancel != nil || s.baseCtx == nil || reader == nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(s.baseCtx)
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
	active := s.shouldPollLocked()
	emit := s.emit
	s.mu.Unlock()

	if !active {
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
	// Only when nothing wants a loop: a hide/show pair that outran this goroutine
	// has already installed a newer cancel that must not be dropped.
	if !s.shouldPollLocked() {
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
