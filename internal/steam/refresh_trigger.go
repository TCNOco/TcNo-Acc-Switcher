package steam

import "sync"

var (
	refreshTriggerMu sync.Mutex
	refreshTrigger   func()

	guardSweepMu      sync.Mutex
	guardSweepTrigger func(force bool)
)

// RegisterProfileRefreshTrigger points RequestProfileRefresh at the app's
// SteamService. main calls it once at startup; left unregistered, as in tests,
// RequestProfileRefresh is a no-op.
func RegisterProfileRefreshTrigger(fn func()) {
	refreshTriggerMu.Lock()
	defer refreshTriggerMu.Unlock()
	refreshTrigger = fn
}

// RequestProfileRefresh kicks the background avatar and persona fetch from code
// that holds no SteamService. Steam Guard enrollment is the caller: it adds
// accounts to the switcher's store, and without this their tiles would sit
// blank until the next natural refresh.
func RequestProfileRefresh() {
	refreshTriggerMu.Lock()
	fn := refreshTrigger
	refreshTriggerMu.Unlock()
	if fn != nil {
		fn()
	}
}

// RegisterSteamGuardSweepTrigger points RequestSteamGuardSweep at the Steam
// Guard service's authenticated sweeps. Left unregistered - in tests, or with
// the feature switched off - the request is a no-op.
func RegisterSteamGuardSweepTrigger(fn func(force bool)) {
	guardSweepMu.Lock()
	defer guardSweepMu.Unlock()
	guardSweepTrigger = fn
}

// RequestSteamGuardSweep asks for the CS2 cooldown, rank and Prime sweep and the
// owned games sweep to run now.
//
// This is the only way to reach the figures that need a signed-in session: a
// CS2 rank comes from an authenticated GCPD read, so a refresh that does not
// come through here leaves the rank at whatever the last sweep managed.
func RequestSteamGuardSweep(force bool) {
	guardSweepMu.Lock()
	fn := guardSweepTrigger
	guardSweepMu.Unlock()
	if fn != nil {
		fn(force)
	}
}
