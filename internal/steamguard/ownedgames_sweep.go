package steamguard

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/crashlog"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/steam"
	"TcNo-Acc-Switcher/internal/steam/ownedgames"
	"TcNo-Acc-Switcher/internal/steamguard/confirmationapi"
	"TcNo-Acc-Switcher/internal/steamguard/sessionrefresh"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

const (
	// ownedGamesSweepInterval is four times the cooldown sweep's, because a
	// library only changes when someone buys a game. A cooldown ticks down on its
	// own; ownership does not.
	ownedGamesSweepInterval = 24 * time.Hour

	// ownedGamesRememberedSweepInterval applies while the vault key is held under
	// a ProcessLease, which is what remembering the password for the session
	// unlocks under. Under the default FixedLease the vault relocks after five
	// minutes and the unlock trigger does the work; a remembered password makes
	// unlocks rare, so the ticker is the only thing that ever fires. It is equal
	// to the per-account floor, so the shorter cadence still cannot cost an
	// account more than one request per floor.
	ownedGamesRememberedSweepInterval = 6 * time.Hour

	// ownedGamesSweepPoll bounds how long the sweeper waits before re-reading the
	// interval. Remember-password can be switched on halfway through a session,
	// and the sweeper must not sit out the 24h wait it started under.
	ownedGamesSweepPoll = 15 * time.Minute

	// ownedGamesAccountFloor is the minimum gap between two requests for the same
	// account. Much longer than the cooldown sweep's 90s for the same reason the
	// interval is: the unlock trigger fires on every unlock, and re-reading a
	// library that changes monthly on each one is pure noise.
	ownedGamesAccountFloor = 6 * time.Hour

	// ownedGamesSweepFloor is the minimum gap between whole sweeps.
	ownedGamesSweepFloor = 90 * time.Second

	// ownedGamesStagger paces the sweep. Steam is not a CDN and the protocol
	// client has no rate limiter, so the discipline is entirely ours.
	ownedGamesStagger = 2 * time.Second

	// Longer than the cooldown request timeout: this is the one call whose
	// response grows with the size of the account.
	ownedGamesRequestTimeout = 60 * time.Second
)

func ownedGamesLogger() *slog.Logger {
	return slog.Default().With("component", "steamguard.ownedgames")
}

// ownedGamesTarget is one account's credentials, lifted out of the vault before
// any network work starts.
//
// tokenLapsed and refreshUsable are verdicts rather than tokens: both are
// decided while the record is still decrypted, so the refresh token never has to
// leave collectOwnedGamesTargets.
type ownedGamesTarget struct {
	steamID64     string
	accessToken   string
	tokenLapsed   bool
	refreshUsable bool
}

type ownedGamesSweepState struct {
	mu        sync.Mutex
	running   bool
	lastSweep time.Time
	cancel    context.CancelFunc
	// wake is buffered so signalOwnedGamesSweep can be called with s.mu held.
	wake  chan struct{}
	retry sweepRetry
}

func (s *Service) startOwnedGamesSweeper(ctx context.Context) {
	s.ownedGamesSweep.mu.Lock()
	if s.ownedGamesSweep.cancel != nil {
		s.ownedGamesSweep.cancel()
	}
	sweepCtx, cancel := context.WithCancel(ctx)
	s.ownedGamesSweep.cancel = cancel
	s.ownedGamesSweep.mu.Unlock()
	go s.runOwnedGamesSweeper(sweepCtx)
}

func (s *Service) stopOwnedGamesSweeper() {
	s.ownedGamesSweep.mu.Lock()
	cancel := s.ownedGamesSweep.cancel
	s.ownedGamesSweep.cancel = nil
	s.ownedGamesSweep.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// signalOwnedGamesSweep asks for a sweep. It is called from inside the unlock
// path with s.mu held, so it must never block.
func (s *Service) signalOwnedGamesSweep() {
	select {
	case s.ownedGamesSweep.wake <- struct{}{}:
	default:
	}
}

func (s *Service) runOwnedGamesSweeper(ctx context.Context) {
	defer crashlog.Capture()
	ticker := time.NewTicker(ownedGamesSweepPoll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.ownedGamesSweep.wake:
		case <-ticker.C:
			if !s.ownedGamesSweepDue(time.Now()) {
				continue
			}
		}
		s.sweepOwnedGames(ctx)
	}
}

// ownedGamesSweepDue reports whether the interval in effect right now has passed
// since the last sweep.
//
// The interval is read per poll rather than once at startup: enabling
// remember-password mid-session moves the vault onto a ProcessLease, and the
// shorter cadence has to reach the sweeper that is already running under the
// longer one.
func (s *Service) ownedGamesSweepDue(now time.Time) bool {
	s.ownedGamesSweep.mu.Lock()
	last := s.ownedGamesSweep.lastSweep
	s.ownedGamesSweep.mu.Unlock()
	return last.IsZero() || now.Sub(last) >= s.ownedGamesSweepIntervalNow()
}

func (s *Service) ownedGamesSweepIntervalNow() time.Duration {
	if s.vaultLeaseMode() == vault.ProcessLease {
		return ownedGamesRememberedSweepInterval
	}
	return ownedGamesSweepInterval
}

// vaultLeaseMode reports how the vault key is held right now. It reads the
// already-open vault rather than requireVaultLocked: a poll must not open a
// vault this process has had no reason to touch.
func (s *Service) vaultLeaseMode() vault.LeaseMode {
	s.mu.Lock()
	v := s.vault
	s.mu.Unlock()
	if v == nil {
		return 0
	}
	return v.LeaseMode()
}

// sweepOwnedGames reads every vault account's Steam library into the plaintext
// owned games store, so the games view can render long after the vault relocks.
//
// Unlike the cooldown sweep this one does write to the vault - it renews lapsed
// sessions first. See refreshLapsedOwnedGamesSessions for why that is affordable
// here and nowhere else.
func (s *Service) sweepOwnedGames(ctx context.Context) {
	log := ownedGamesLogger()

	s.ownedGamesSweep.mu.Lock()
	if s.ownedGamesSweep.running {
		s.ownedGamesSweep.mu.Unlock()
		log.Debug("sweep skipped: already running")
		return
	}
	if since := time.Since(s.ownedGamesSweep.lastSweep); !s.ownedGamesSweep.lastSweep.IsZero() && since < ownedGamesSweepFloor {
		s.ownedGamesSweep.mu.Unlock()
		log.Debug("sweep skipped: rate limited", "since", since)
		return
	}
	s.ownedGamesSweep.running = true
	s.ownedGamesSweep.mu.Unlock()
	defer func() {
		s.ownedGamesSweep.mu.Lock()
		s.ownedGamesSweep.running = false
		s.ownedGamesSweep.lastSweep = time.Now()
		s.ownedGamesSweep.mu.Unlock()
	}()

	if ctx.Err() != nil || security.AppLocked() || appclient.IsOfflineMode() {
		// Nothing a retry would fix, so a pending one is dropped rather than
		// left to wake a sweep that will bail here again.
		s.ownedGamesSweep.retry.cancel()
		log.Debug("sweep skipped: app locked, offline, or cancelled")
		return
	}
	guardSettings, err := LoadSettings()
	if err != nil || !guardSettings.FeatureEnabled {
		log.Debug("sweep skipped: Steam Guard feature disabled")
		return
	}
	if s.confirmationClient == nil {
		return
	}

	targets := s.collectOwnedGamesTargets(time.Now())
	if len(targets) == 0 {
		return
	}
	stored, err := ownedgames.Load()
	if err != nil {
		log.Warn("owned games store unreadable; starting from empty", "error", err)
		stored = map[string]ownedgames.Entry{}
	}

	// Only accounts that are actually due are worth renewing: a lapsed session
	// still inside its per-account floor would otherwise drag the whole vault
	// through a generation switch to fetch a library nobody is going to read.
	var lapsed []uint64
	for _, target := range targets {
		if !target.tokenLapsed || !target.refreshUsable ||
			!ownedGamesDue(stored, target.steamID64, time.Now()) {
			continue
		}
		steamID, parseErr := strconv.ParseUint(target.steamID64, 10, 64)
		if parseErr != nil {
			continue
		}
		lapsed = append(lapsed, steamID)
	}
	if len(lapsed) > 0 {
		s.refreshLapsedOwnedGamesSessions(ctx, lapsed)
		// The batch rewrote the vault, so every token lifted before it is the old
		// one. Re-collecting is what makes the renewal worth doing at all.
		targets = s.collectOwnedGamesTargets(time.Now())
	}

	log.Info("owned games sweep started", "accounts", len(targets))
	checked := 0
	// As in the cooldown sweep: one unreachable account means the network is
	// down for all of them, and a day-long cadence is far too long to wait it
	// out.
	unreachable := false
	for _, target := range targets {
		if ctx.Err() != nil {
			return
		}
		if appclient.IsOfflineMode() || security.AppLocked() {
			return
		}
		now := time.Now()
		if !ownedGamesDue(stored, target.steamID64, now) {
			continue
		}
		// A locally readable expiry saves a request that would only be answered
		// with an empty library, which is indistinguishable from owning nothing.
		if target.accessToken == "" || target.tokenLapsed {
			log.Debug("skipping account with no usable session", "steamId64", target.steamID64)
			continue
		}
		// Staggered here, not at the top of the loop: the stagger exists to space
		// out requests to Steam, and an account skipped above makes none.
		if checked > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(ownedGamesStagger):
			}
		}
		if retryableSweepFailure(s.fetchAndStoreOwnedGames(ctx, target, now)) {
			unreachable = true
		}
		checked++
	}
	log.Info("owned games sweep finished", "checked", checked)
	if delay, failures := s.ownedGamesSweep.retry.note(unreachable, s.signalOwnedGamesSweep); delay > 0 {
		log.Info("owned games sweep could not reach Steam; retrying",
			"consecutiveFailures", failures, "in", delay)
	}
}

// ownedGamesDue reports whether an account is outside its per-account floor.
// Shared by the renewal pass and the fetch loop so the two cannot disagree about
// which accounts this sweep is for.
func ownedGamesDue(stored map[string]ownedgames.Entry, steamID64 string, now time.Time) bool {
	entry, ok := stored[steamID64]
	return !ok || now.Sub(time.Unix(entry.CheckedAt, 0)) >= ownedGamesAccountFloor
}

// refreshLapsedOwnedGamesSessions renews every lapsed account in one batch.
//
// This is the one deliberate difference from the cooldown sweep, which is
// forbidden from writing at all. Each vault generation switch invalidates every
// capability outstanding against it, including the one an open Steam Guard
// window holds - so N accounts refreshed one at a time cost N invalidations,
// while RefreshBatch costs exactly one for the whole list. An access token lasts
// about a day and this sweep runs daily, so without the renewal the store would
// only ever fill for accounts someone happened to open by hand.
func (s *Service) refreshLapsedOwnedGamesSessions(ctx context.Context, lapsed []uint64) {
	log := ownedGamesLogger()

	// Re-tested here rather than relying on the check at the top of the sweep:
	// collecting targets is not instant, and a renewal is a Steam token call per
	// account. RefreshBatch gates itself as well, so a flip during the batch stops
	// it too.
	if appclient.IsOfflineMode() {
		log.Debug("session refresh skipped: offline mode", "accounts", len(lapsed))
		return
	}

	s.mu.Lock()
	v, err := s.requireVaultLocked()
	newRefresher := s.newSessionRefresher
	s.mu.Unlock()
	if err != nil || v == nil || v.IsLocked() || newRefresher == nil {
		log.Debug("session refresh skipped: no unlocked vault", "accounts", len(lapsed))
		return
	}

	results, err := newRefresher(v).RefreshBatch(ctx, lapsed)
	if err != nil {
		log.Warn("owned games session refresh failed", "accounts", len(lapsed), "error", err)
		return
	}
	log.Info("owned games sessions refreshed in one vault generation",
		"requested", len(lapsed), "refreshed", len(results))
}

// fetchAndStoreOwnedGames reads one account's library. The returned error is for
// the sweep's retry decision only - every outcome is already logged here.
func (s *Service) fetchAndStoreOwnedGames(ctx context.Context, target ownedGamesTarget, now time.Time) error {
	log := ownedGamesLogger()
	requestCtx, cancel := context.WithTimeout(ctx, ownedGamesRequestTimeout)
	appIDs, err := s.confirmationClient.FetchOwnedApps(requestCtx, confirmationapi.Credentials{
		SteamID:     target.steamID64,
		AccessToken: target.accessToken,
	})
	cancel()
	if err != nil {
		var apiErr *confirmationapi.Error
		if errors.As(err, &apiErr) && apiErr.Kind == confirmationapi.FailureReauth {
			// Steam answers a caller it will not authorise with an empty library
			// rather than an error, so this is the token being refused, not an
			// account that owns nothing. Storing either would leave the account
			// permanently blank in the games view.
			log.Debug("owned games request was refused", "steamId64", target.steamID64)
			return err
		}
		log.Debug("owned games fetch failed", "steamId64", target.steamID64, "error", err)
		return err
	}
	if err := ownedgames.Put(target.steamID64, appIDs, now); err != nil {
		log.Warn("owned games could not be stored", "steamId64", target.steamID64, "error", err)
		return err
	}
	s.emitOwnedGamesUpdate(target.steamID64, len(appIDs))
	return nil
}

// collectOwnedGamesTargets lifts every account's credentials out of the vault in
// one pass.
//
// Splitting the sweep this way is what makes it robust: the service lock is held
// for a few milliseconds of decryption and nothing else, so lease expiry, a
// generation rotation, Lock Now or an app lock partway through cannot break a
// sweep that is already running - it simply has no vault work left to do.
func (s *Service) collectOwnedGamesTargets(now time.Time) []ownedGamesTarget {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.requireVaultLocked()
	if err != nil || v.IsLocked() {
		return nil
	}
	records, err := v.List()
	if err != nil {
		return nil
	}
	targets := make([]ownedGamesTarget, 0, len(records))
	for _, record := range records {
		loaded, err := recordFromVault(v, record.ID)
		if err != nil {
			continue
		}
		accessToken := loaded.AccessToken()
		target := ownedGamesTarget{
			steamID64:   record.SteamID64,
			accessToken: accessToken,
			tokenLapsed: accessToken == "" ||
				sessionrefresh.AccessTokenExpired(accessToken, now, accessTokenSkew),
			refreshUsable: sessionrefresh.RefreshTokenUsable(loaded.RefreshToken(), now, accessTokenSkew),
		}
		loaded.destroy()
		targets = append(targets, target)
	}
	return targets
}

func (s *Service) emitOwnedGamesUpdate(steamID64 string, appCount int) {
	if s.emitOwnedGamesFn == nil {
		return
	}
	s.emitOwnedGamesFn(steam.OwnedGamesPatch{SteamID64: steamID64, AppCount: appCount})
}
