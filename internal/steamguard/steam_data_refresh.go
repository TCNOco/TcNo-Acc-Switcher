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
// it.
//
// force is for the add: it skips the cooldown sweep's rate limit so an account
// that has never been checked is checked, without also restarting the bulk
// download that the unlock a minute ago already did. It never blocks, because
// the unlock path calls it with s.mu held.
func (s *Service) signalSteamDataRefresh(force bool) {
	s.signalSteamGuardSweeps(force)
	if !s.steamDataRefreshDue() {
		return
	}
	basic.ForceGameStatsRefresh(steam.PlatformKey)
	steam.RequestProfileRefresh()
}

// signalSteamGuardSweeps wakes only the two sweeps that need a signed-in
// session, without the bulk stats and profile download around them.
//
// This is what the account page's refresh reaches: it already re-fetches images,
// names and game stats itself, but a CS2 rank is not something it can ask for -
// the number comes from an authenticated GCPD read, and this is the only way to
// start one.
func (s *Service) signalSteamGuardSweeps(force bool) {
	s.signalCooldownSweep(force)
	s.signalOwnedGamesSweep()
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
