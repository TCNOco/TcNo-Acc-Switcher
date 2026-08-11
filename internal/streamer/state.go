// Package streamer decides when the app should hide account identifiers because
// the screen is being broadcast, and supplies the machine-local salt that makes
// generated stand-in avatars unreproducible on any other computer.
package streamer

import "sync"

// State is everything the UI needs in one shot: the two stored preferences, what
// detection currently sees, and the answer that actually drives rendering.
type State struct {
	// Manual is the user's override toggle. On means always censor.
	Manual bool `json:"manual"`
	// AutoEnabled is the "watch for broadcasting software" preference.
	AutoEnabled bool `json:"autoEnabled"`
	// AutoActive is true while at least one known broadcaster is running. It stays
	// meaningful even with AutoEnabled off so the UI can explain itself.
	AutoActive bool `json:"autoActive"`
	// DetectedExe is the image name that turned AutoActive on, for the UI hint.
	DetectedExe string `json:"detectedExe"`
	// Effective is Manual || (AutoEnabled && AutoActive).
	Effective bool `json:"effective"`
	// AvatarSalt seeds generated avatars; see MachineSalt.
	AvatarSalt string `json:"avatarSalt"`
}

var state struct {
	mu          sync.Mutex
	manual      bool
	autoEnabled bool
	autoActive  bool
	detectedExe string
	hook        func(State)
}

// watchFn is the detector entry point, indirected so tests can exercise the state
// machine without installing real Win32 hooks. Assigned in init rather than at
// declaration: setWatching reaches back into apply, and Go rejects that as an
// initialisation cycle.
var watchFn func(bool)

func init() { watchFn = setWatching }

// SetChangeHook registers the single listener notified whenever the effective
// state changes. Registered by internal/platform, which owns the Wails event.
func SetChangeHook(fn func(State)) {
	state.mu.Lock()
	state.hook = fn
	state.mu.Unlock()
}

// snapshotLocked must be called with the lock held.
func snapshotLocked() State {
	return State{
		Manual:      state.manual,
		AutoEnabled: state.autoEnabled,
		AutoActive:  state.autoActive,
		DetectedExe: state.detectedExe,
		Effective:   state.manual || (state.autoEnabled && state.autoActive),
		AvatarSalt:  MachineSalt(),
	}
}

// Current returns the state as the UI should see it.
func Current() State {
	state.mu.Lock()
	defer state.mu.Unlock()
	return snapshotLocked()
}

// apply mutates state under the lock and fires the hook outside it, but only when
// the mutation actually changed something the frontend would render differently.
func apply(mutate func()) {
	state.mu.Lock()
	before := snapshotLocked()
	mutate()
	after := snapshotLocked()
	hook := state.hook
	watch := after.AutoEnabled
	state.mu.Unlock()

	watchFn(watch)

	if hook != nil && before != after {
		hook(after)
	}
}

// Init seeds the stored preferences at startup and starts detection if asked.
func Init(manual, autoEnabled bool) {
	apply(func() {
		state.manual = manual
		state.autoEnabled = autoEnabled
	})
}

// SetManual records the override toggle.
func SetManual(enabled bool) {
	apply(func() { state.manual = enabled })
}

// SetAutoEnabled records the auto toggle and starts or stops detection.
func SetAutoEnabled(enabled bool) {
	apply(func() {
		state.autoEnabled = enabled
		if !enabled {
			// Stale detection results would otherwise be reported back the moment
			// the toggle is turned on again, before the watcher has looked.
			state.autoActive = false
			state.detectedExe = ""
		}
	})
}

// setDetected is called by the platform watcher when broadcasters appear or go away.
func setDetected(active bool, exe string) {
	apply(func() {
		state.autoActive = active
		if active {
			state.detectedExe = exe
		} else {
			state.detectedExe = ""
		}
	})
}

// Shutdown stops detection. Safe to call when nothing was started.
func Shutdown() {
	watchFn(false)
}
