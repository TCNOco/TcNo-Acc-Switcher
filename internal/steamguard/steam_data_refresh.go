package steamguard

import (
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/basic"
	"TcNo-Acc-Switcher/internal/steam"
)

// steamDataRefreshFloor is the minimum gap between two whole-platform refreshes.
//
// Under the default FixedLease the vault relocks after five minutes, so a user
// who does not remember the password for the session unlocks it again and
// again; without a floor each of those would restart every account's stats
// download. The sweeps next door settled on the same 90s for the same reason.
const steamDataRefreshFloor = 90 * time.Second

type steamDataRefreshState struct {
	mu   sync.Mutex
	last time.Time
}

// signalSteamDataRefresh brings every Steam-sourced figure forward to now.
//
// Unlocking the vault is the moment the app can answer questions it could not a
// second earlier - CS2 cooldowns, ranks and Prime all come from an authenticated
// sweep - and adding an account is the moment a tile exists with nothing behind
// it. Either way the right answer is current data, not whatever survives until
// the next cache lifetime expires hours from now.
//
// force is for the add: it skips the cooldown sweep's rate limit so an account
// that has never been checked is checked, without also restarting the bulk
// download that the unlock a minute ago already did. It never blocks, because
// the unlock path calls it with s.mu held.
func (s *Service) signalSteamDataRefresh(force bool) {
	s.signalCooldownSweep(force)
	s.signalOwnedGamesSweep()
	if !s.steamDataRefreshDue() {
		return
	}
	basic.ForceGameStatsRefresh(steam.PlatformKey)
	steam.RequestProfileRefresh()
}

func (s *Service) steamDataRefreshDue() bool {
	now := time.Now()
	s.steamDataRefresh.mu.Lock()
	defer s.steamDataRefresh.mu.Unlock()
	if !s.steamDataRefresh.last.IsZero() && now.Sub(s.steamDataRefresh.last) < steamDataRefreshFloor {
		return false
	}
	s.steamDataRefresh.last = now
	return true
}
