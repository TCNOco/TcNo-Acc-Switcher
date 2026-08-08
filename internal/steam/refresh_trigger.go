package steam

import "sync"

var (
	refreshTriggerMu sync.Mutex
	refreshTrigger   func()
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
