package platform

import (
	"sync"
	"sync/atomic"

	"TcNo-Acc-Switcher/internal/foreground"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ScreenCoveredEvent carries true while a fullscreen application from another
// process is in front, and false once it is not.
//
// The account list draws Steam's animated avatar frames — multi-megabyte APNGs
// and GIFs that keep decoding for as long as they are in the document, whether
// or not the window is visible. Behind a game that is pure waste, so the UI
// suspends them on true and restores them on false.
const ScreenCoveredEvent = "screen-covered"

var screenCover struct {
	mu       sync.Mutex
	stop     func()
	stopGame func()
}

// GameRunningEvent carries true while Steam has a game running, windowed or
// not. Fullscreen detection cannot see a windowed game, and a windowed game is
// still a reason not to spend a core animating decorations.
const GameRunningEvent = "game-running"

// Latest state, so a page mounting after the event fired still gets the truth.
var (
	screenCoverState atomic.Bool
	gameRunningState atomic.Bool
)

// GetScreenCovered reports whether a fullscreen application is currently in
// front. The UI calls this once on mount and follows ScreenCoveredEvent after.
func (p *PlatformService) GetScreenCovered() (bool, error) {
	return screenCoverState.Load(), nil
}

// GetGameRunning reports whether Steam currently has a game running.
func (p *PlatformService) GetGameRunning() (bool, error) {
	return gameRunningState.Load(), nil
}

// StartScreenCoverWatch begins reporting fullscreen coverage to the frontend.
// Idempotent; safe to call before any window exists, since the first event is
// re-sent on request via GetScreenCovered.
func StartScreenCoverWatch() {
	screenCover.mu.Lock()
	defer screenCover.mu.Unlock()
	if screenCover.stop != nil {
		return
	}
	screenCover.stop = foreground.Watch(func(covered bool) {
		screenCoverState.Store(covered)
		emitEvent(ScreenCoveredEvent, covered)
	})
	screenCover.stopGame = foreground.WatchSteamGame(func(running bool) {
		gameRunningState.Store(running)
		emitEvent(GameRunningEvent, running)
	})
}

func emitEvent(name string, value bool) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(name, value)
}

// StopScreenCoverWatch releases the hooks. Called on shutdown.
func StopScreenCoverWatch() {
	screenCover.mu.Lock()
	stop, stopGame := screenCover.stop, screenCover.stopGame
	screenCover.stop, screenCover.stopGame = nil, nil
	screenCover.mu.Unlock()
	if stop != nil {
		stop()
	}
	if stopGame != nil {
		stopGame()
	}
}
