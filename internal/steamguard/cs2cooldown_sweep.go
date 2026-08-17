package steamguard

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/basic"
	"TcNo-Acc-Switcher/internal/crashlog"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/steam"
	"TcNo-Acc-Switcher/internal/steamguard/confirmationapi"
	"TcNo-Acc-Switcher/internal/steamguard/cs2cooldown"
	"TcNo-Acc-Switcher/internal/steamguard/cs2ranks"
	"TcNo-Acc-Switcher/internal/steamguard/gcpd"
	"TcNo-Acc-Switcher/internal/steamguard/primestatus"
	"TcNo-Acc-Switcher/internal/steamguard/sessionrefresh"
)

const (
	// cooldownSweepInterval re-checks while the vault stays unlocked. Under the
	// default FixedLease the vault relocks after five minutes, so in practice
	// this only fires for users who chose to remember the password for the
	// session; everyone else gets the unlock-triggered sweep.
	cooldownSweepInterval = 6 * time.Hour

	// cooldownAccountFloor is the minimum gap between two requests for the same
	// account. Re-unlocking repeatedly must not turn into repeated requests.
	cooldownAccountFloor = 90 * time.Second

	// cooldownSweepFloor is the minimum gap between whole sweeps.
	cooldownSweepFloor = 90 * time.Second

	// cooldownAccountStagger paces the sweep. Steam is not a CDN and the
	// protocol client has no rate limiter, so the discipline is entirely ours.
	cooldownAccountStagger = 2 * time.Second

	cooldownRequestTimeout = 30 * time.Second
)

func cooldownLogger() *slog.Logger {
	return slog.Default().With("component", "steamguard.cs2cooldown")
}

// cooldownTarget is one account's credentials, lifted out of the vault before
// any network work starts.
type cooldownTarget struct {
	steamID64   string
	accessToken string
	sessionID   string
}

type cooldownSweepState struct {
	mu        sync.Mutex
	running   bool
	lastSweep time.Time
	cancel    context.CancelFunc
	// wake is buffered so signalCooldownSweep can be called with s.mu held.
	wake chan struct{}
	// forced survives a skipped sweep so a wake that must not be rate limited -
	// an account joining the vault - still gets one. Atomic rather than under
	// mu: signalCooldownSweep already runs with the service lock held.
	forced atomic.Bool
	retry  sweepRetry
}

func (s *Service) startCooldownSweeper(ctx context.Context) {
	s.cooldownSweep.mu.Lock()
	if s.cooldownSweep.cancel != nil {
		s.cooldownSweep.cancel()
	}
	sweepCtx, cancel := context.WithCancel(ctx)
	s.cooldownSweep.cancel = cancel
	s.cooldownSweep.mu.Unlock()
	go s.runCooldownSweeper(sweepCtx)
}

func (s *Service) stopCooldownSweeper() {
	s.cooldownSweep.mu.Lock()
	cancel := s.cooldownSweep.cancel
	s.cooldownSweep.cancel = nil
	s.cooldownSweep.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// signalCooldownSweep asks for a sweep. It is called from inside the unlock
// path with s.mu held, so it must never block.
//
// force skips the whole-sweep floor. An account that has just joined the vault
// has never been checked, so the floor - which exists to stop repeated unlocks
// re-checking the same accounts - would otherwise be answering a question that
// was not asked.
func (s *Service) signalCooldownSweep(force bool) {
	if force {
		s.cooldownSweep.forced.Store(true)
	}
	select {
	case s.cooldownSweep.wake <- struct{}{}:
	default:
	}
}

func (s *Service) runCooldownSweeper(ctx context.Context) {
	defer crashlog.Capture()
	ticker := time.NewTicker(cooldownSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.cooldownSweep.wake:
		case <-ticker.C:
		}
		s.sweepCS2Cooldowns(ctx)
	}
}

// sweepCS2Cooldowns reads every vault account's CS2 cooldown.
//
// It is strictly read-only against the vault. Anything that writes - notably
// refreshing an access token - rotates the vault generation and invalidates
// every outstanding capability, including the one the open Steam Guard window is
// holding, so a background sweep must never do it.
func (s *Service) sweepCS2Cooldowns(ctx context.Context) {
	log := cooldownLogger()

	// Read, not consumed: a sweep that is skipped below must leave the request
	// standing, or the account it was raised for is never checked.
	forced := s.cooldownSweep.forced.Load()

	s.cooldownSweep.mu.Lock()
	if s.cooldownSweep.running {
		s.cooldownSweep.mu.Unlock()
		log.Debug("sweep skipped: already running")
		return
	}
	if since := time.Since(s.cooldownSweep.lastSweep); !forced && !s.cooldownSweep.lastSweep.IsZero() && since < cooldownSweepFloor {
		s.cooldownSweep.mu.Unlock()
		log.Debug("sweep skipped: rate limited", "since", since)
		return
	}
	s.cooldownSweep.running = true
	s.cooldownSweep.mu.Unlock()
	// Stamped only once the vault has actually handed over accounts to read.
	//
	// The floor exists to space out requests to Steam, and every bail below makes
	// none - the commonest by far being a locked vault, which yields no targets.
	// Stamping those anyway is what made unlocking the vault appear to do nothing:
	// the account page's refresh a moment earlier had already found the vault
	// locked, done no work, and put the next ninety seconds out of bounds, so the
	// unlock - the one event that changes what a sweep can answer - was turned
	// away as rate limited and no rank ever moved.
	swept := false
	defer func() {
		s.cooldownSweep.mu.Lock()
		s.cooldownSweep.running = false
		if swept {
			s.cooldownSweep.lastSweep = time.Now()
		}
		s.cooldownSweep.mu.Unlock()
	}()

	if ctx.Err() != nil || security.AppLocked() || appclient.IsOfflineMode() {
		// Nothing a retry would fix, so a pending one is dropped rather than
		// left to wake a sweep that will bail here again.
		s.cooldownSweep.retry.cancel()
		log.Debug("sweep skipped: app locked, offline, or cancelled")
		return
	}
	steamSettings, err := steam.LoadSettings()
	// Any one of the three is reason enough to run: they read the same GCPD
	// response, so the cooldown setting no longer gates the other two.
	if err != nil || !(steamSettings.SteamCollectCS2Cooldowns ||
		steamSettings.SteamShowCS2Rank ||
		steamSettings.SteamShowCS2PrimeTag) {
		log.Debug("sweep skipped: disabled in Steam settings")
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

	targets := s.collectCooldownTargets()
	if len(targets) == 0 {
		return
	}
	swept = true
	// Consumed here rather than at the top, for the same reason: a force raised so
	// a newly added account gets checked has to survive a run that never reached
	// one, or the add is answered by a sweep that does nothing and the request is
	// gone.
	s.cooldownSweep.forced.Store(false)
	stored, err := cs2cooldown.Load()
	if err != nil {
		log.Warn("cooldown store unreadable; starting from empty", "error", err)
		stored = map[string]cs2cooldown.Entry{}
	}
	log.Info("CS2 cooldown sweep started", "accounts", len(targets))

	checked := 0
	// One verdict for the whole sweep: every account goes to the same Steam over
	// the same adapter, so any of them reporting "temporarily failed" means the
	// sweep is worth repeating rather than left until the six-hour tick.
	unreachable := false
	for _, target := range targets {
		if ctx.Err() != nil {
			return
		}
		if appclient.IsOfflineMode() || security.AppLocked() {
			return
		}
		now := time.Now()
		if entry, ok := stored[target.steamID64]; ok && now.Sub(time.Unix(entry.CheckedAt, 0)) < cooldownAccountFloor {
			continue
		}
		// A locally readable expiry saves a request that would only be answered
		// with a sign-in page.
		if sessionrefresh.AccessTokenExpired(target.accessToken, now, accessTokenSkew) {
			log.Debug("skipping account with a lapsed session", "steamId64", target.steamID64)
			continue
		}
		// Staggered here, not at the top of the loop: the stagger exists to space
		// out requests to Steam, and an account skipped above makes none. Pacing
		// them anyway made a sweep of mostly-lapsed accounts take seconds per
		// account to do nothing.
		if checked > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(cooldownAccountStagger):
			}
		}
		err := s.fetchAndStoreCooldown(ctx, target, now, steamSettings.SteamShowCS2PrimeTag)
		if err == errCooldownRateLimited {
			// Walking the rest of the list into the same wall is exactly the
			// pattern that earns a longer ban. The next unlock retries.
			log.Warn("CS2 cooldown sweep aborted: rate limited by Steam", "checked", checked)
			return
		}
		if retryableSweepFailure(err) {
			unreachable = true
		}
		checked++
	}
	log.Info("CS2 cooldown sweep finished", "checked", checked)
	if delay, failures := s.cooldownSweep.retry.note(unreachable, func() {
		s.signalCooldownSweep(true)
	}); delay > 0 {
		log.Info("CS2 cooldown sweep could not reach Steam; retrying",
			"consecutiveFailures", failures, "in", delay)
	}
}

var errCooldownRateLimited = errors.New("rate limited")

// logCooldownFetchFailure records why one account was not read, at a level that
// matches how surprising it is.
//
// This was a flat Debug line, and that is how a vanity-URL redirect went
// unnoticed: every account with a custom Steam URL failed here, the sweep
// counted them as checked, and nothing above Debug ever said otherwise. Only the
// app's own state is quiet now; anything else names its kind and status, so a
// failure with no status - a request that got no answer at all, the shape a
// denied redirect has - is legible rather than invisible.
func logCooldownFetchFailure(log *slog.Logger, steamID64 string, err error) {
	attributes := []any{"steamId64", steamID64}
	var apiErr *confirmationapi.Error
	if !errors.As(err, &apiErr) {
		log.Warn("cooldown fetch failed", append(attributes, "error", err)...)
		return
	}
	attributes = append(attributes, "kind", string(apiErr.Kind))
	if apiErr.StatusCode != 0 {
		attributes = append(attributes, "status", apiErr.StatusCode)
	}
	if apiErr.Detail != "" {
		attributes = append(attributes, "detail", apiErr.Detail)
	}
	switch apiErr.Kind {
	case confirmationapi.FailureOffline, confirmationapi.FailureCanceled:
		// The app going offline or shutting down mid-sweep. Says nothing about
		// the account.
		log.Debug("cooldown fetch skipped", attributes...)
	case confirmationapi.FailureReauth:
		// The stored session was refused. Expected on its own - the locally
		// readable expiry is checked first, so reaching here means Steam
		// disagreed - but worth seeing, because it is also where a redirect the
		// transport would not follow ends up.
		log.Info("cooldown fetch needs re-authentication", attributes...)
	default:
		log.Warn("cooldown fetch failed", append(attributes, "error", err)...)
	}
}

func (s *Service) fetchAndStoreCooldown(
	ctx context.Context, target cooldownTarget, now time.Time, collectPrime bool,
) error {
	log := cooldownLogger()
	credentials := confirmationapi.Credentials{
		SteamID:     target.steamID64,
		AccessToken: target.accessToken,
		SessionID:   target.sessionID,
	}
	requestCtx, cancel := context.WithTimeout(ctx, cooldownRequestTimeout)
	body, err := s.confirmationClient.FetchCS2GCPD(requestCtx, credentials)
	cancel()
	if err != nil {
		var apiErr *confirmationapi.Error
		if errors.As(err, &apiErr) && apiErr.Kind == confirmationapi.FailureRateLimit {
			return errCooldownRateLimited
		}
		logCooldownFetchFailure(log, target.steamID64, err)
		return err
	}

	result := gcpd.Parse(body, now)
	if result.Outcome != gcpd.OutcomeParsed {
		// Not understood, or a sign-in page. Write nothing and stamp nothing: a
		// stored cooldown stays correct on its own, and clearing it on the
		// strength of a page we could not read would delete real information.
		log.Debug("cooldown response not usable",
			"steamId64", target.steamID64, "outcome", result.Outcome.String())
		return nil
	}
	if result.HasCooldown {
		err = cs2cooldown.Put(target.steamID64, result.ExpiresAt, result.Permanent, now)
	} else {
		err = cs2cooldown.Clear(target.steamID64, now)
	}
	if err != nil {
		log.Warn("cooldown could not be stored", "steamId64", target.steamID64, "error", err)
		return err
	}
	primeState := s.primeStateFor(ctx, credentials, result, collectPrime)
	s.storeRanks(target.steamID64, result, primeState, now)
	s.syncCooldownTag(target.steamID64, result)
	// Read back rather than reusing primeState: storeRanks carries the previous
	// verdict forward when this run could not reach one, and the tag has to match
	// what was stored or the two would disagree.
	if stored, ok := cs2ranks.Lookup(target.steamID64); ok {
		s.syncPrimeTags(target.steamID64, stored.PrimeState, collectPrime)
	} else {
		s.syncPrimeTags(target.steamID64, primeState, collectPrime)
	}
	s.emitCooldownPatch(target.steamID64, result)
	// The rank is drawn by the game stats row, which is refreshed by its own
	// event rather than by the cooldown patch. Without this a new rank waits for
	// the next full page load.
	basic.EmitGameStatsUpdated(steam.PlatformKey, target.steamID64)
	// ...and the row itself is re-run, because the ranks just stored are what
	// variant 0 of the CS2 chain reads. Announcing the change without it only
	// re-reads the cache entry a third-party provider last wrote, so an account
	// with CS2 stats configured kept yesterday's number until its own cache
	// lifetime expired hours later.
	basic.QueueGameStatsRefresh(steam.PlatformKey, CS2GameName, target.steamID64)
	return nil
}

// primeStateFor decides Prime for one account, spending a second request only
// when it could change the answer.
//
// Premier history alone proves Prime, so an account that plays Premier never
// costs the extra fetch. Everything else needs the store page, since that is the
// only place package ownership shows.
func (s *Service) primeStateFor(
	ctx context.Context,
	credentials confirmationapi.Credentials,
	result gcpd.Result,
	collectPrime bool,
) string {
	if !collectPrime {
		return PrimeStateUnknown
	}
	if state := decidePrimeState(primestatus.Result{}, result.Ranks, result.HasGameData); state == PrimeStatePrime {
		return state
	}
	// Prime is never revoked, so a confirmed account never needs asking again.
	// Non-Prime is not settled the same way - it can be bought at any time - so
	// only the positive answer is treated as final.
	if entry, ok := cs2ranks.Lookup(credentials.SteamID); ok && entry.PrimeState == PrimeStatePrime {
		return PrimeStatePrime
	}
	requestCtx, cancel := context.WithTimeout(ctx, cooldownRequestTimeout)
	body, err := s.confirmationClient.FetchCS2StorePage(requestCtx, credentials)
	cancel()
	if err != nil {
		// Not a verdict: leaving it unknown keeps whatever was stored before
		// rather than relabelling the account off a failed request.
		cooldownLogger().Debug("store page fetch failed",
			"steamId64", credentials.SteamID, "error", err)
		return PrimeStateUnknown
	}
	return decidePrimeState(primestatus.Parse(body), result.Ranks, result.HasGameData)
}

// storeRanks keeps the Premier and Wingman standings that rode along in the same
// response, so the account list can prefer Valve's own numbers over a
// third-party provider's.
//
// Only ever called from the OutcomeParsed path. A page with no rank table writes
// nothing rather than zeroing: an account that has simply never played Wingman
// must not overwrite a rank we already read.
func (s *Service) storeRanks(steamID64 string, result gcpd.Result, primeState string, now time.Time) {
	// An account can have a Prime verdict and no ranks at all, so the two
	// reasons to write are checked separately.
	if !result.Ranks.Any() && primeState == PrimeStateUnknown {
		return
	}
	entry := cs2ranks.Entry{
		PremierRating: -1,
		PremierWins:   -1,
		WingmanRank:   -1,
		WingmanWins:   -1,
		CompRank:      -1,
		PrimeState:    primeState,
	}
	// A verdict we could not reach this time must not erase the last one we
	// could: the store request fails for transport reasons that say nothing
	// about the account, and Prime changes far more rarely than a rank does.
	if primeState == PrimeStateUnknown {
		if previous, ok := cs2ranks.Lookup(steamID64); ok {
			entry.PrimeState = previous.PrimeState
		}
	}
	if result.Ranks.Premier.Found {
		entry.PremierRating = result.Ranks.Premier.Value
		entry.PremierWins = result.Ranks.Premier.Wins
	}
	if result.Ranks.Wingman.Found {
		entry.WingmanRank = result.Ranks.Wingman.Value
		entry.WingmanWins = result.Ranks.Wingman.Wins
	}
	if result.Ranks.Competitive.Found {
		entry.CompRank = result.Ranks.Competitive.Value
	}
	if err := cs2ranks.Put(steamID64, entry, now); err != nil {
		cooldownLogger().Warn("CS2 ranks could not be stored",
			"steamId64", steamID64, "error", err)
	}
}

// syncCooldownTag mirrors the cooldown onto a tag, so the account list can be
// filtered by it rather than read one tile at a time.
//
// Only ever called from the OutcomeParsed path: a page we could not read must
// leave the tag alone, or one lapsed session would untag every account at once.
// A tag failure is logged and swallowed - the cooldown itself is already stored,
// and the account list must not depend on ids.json being writable.
func (s *Service) syncCooldownTag(steamID64 string, result gcpd.Result) {
	expiry := ""
	// A permanent cooldown gets no expiry, so nothing prunes it away.
	if result.HasCooldown && !result.Permanent {
		expiry = result.ExpiresAt.UTC().Format(time.RFC3339)
	}
	if err := basic.SetManagedTag(
		steam.PlatformKey, steamID64, basic.ManagedTagCS2Cooldown, result.HasCooldown, expiry,
	); err != nil {
		cooldownLogger().Warn("cooldown tag could not be updated",
			"steamId64", steamID64, "error", err)
	}
}

// syncPrimeTags mirrors the Prime verdict onto a pair of managed tags.
//
// Exactly one of the two is ever assigned, so switching state has to remove the
// other rather than only add the new one. An unknown verdict removes both: no
// tag is how "we do not know" is expressed, since a tag can only say something
// positive. Neither carries an expiry - Prime does not lapse on a timer, and
// the next sweep is what revises it.
func (s *Service) syncPrimeTags(steamID64, state string, collectPrime bool) {
	if !collectPrime {
		return
	}
	for tag, on := range map[string]bool{
		basic.ManagedTagCS2Prime:    state == PrimeStatePrime,
		basic.ManagedTagCS2NonPrime: state == PrimeStateNonPrime,
	} {
		if err := basic.SetManagedTag(steam.PlatformKey, steamID64, tag, on, ""); err != nil {
			cooldownLogger().Warn("Prime tag could not be updated",
				"steamId64", steamID64, "tag", tag, "error", err)
		}
	}
}

// collectCooldownTargets lifts every account's credentials out of the vault in
// one pass.
//
// Splitting the sweep this way is what makes it robust: the service lock is held
// for a few milliseconds of decryption and nothing else, so lease expiry, a
// generation rotation, Lock Now or an app lock partway through cannot break a
// sweep that is already running - it simply has no vault work left to do.
func (s *Service) collectCooldownTargets() []cooldownTarget {
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
	targets := make([]cooldownTarget, 0, len(records))
	for _, record := range records {
		loaded, err := recordFromVault(v, record.ID)
		if err != nil {
			continue
		}
		accessToken := loaded.AccessToken()
		sessionID := loaded.SessionID()
		loaded.destroy()
		if accessToken == "" {
			continue
		}
		if sessionID == "" {
			// SDA-imported records carry no sessionid. Steam accepts any
			// client-chosen value, so mint one for this sweep only; persisting
			// it would rotate the vault generation.
			minted, err := newSessionID()
			if err != nil {
				continue
			}
			sessionID = minted
		}
		targets = append(targets, cooldownTarget{
			steamID64:   record.SteamID64,
			accessToken: accessToken,
			sessionID:   sessionID,
		})
	}
	return targets
}

func (s *Service) emitCooldownPatch(steamID64 string, result gcpd.Result) {
	if s.emitCooldownFn == nil {
		return
	}
	patch := steam.CS2CooldownPatch{SteamID64: steamID64, CS2CooldownPermanent: result.Permanent}
	if result.HasCooldown && !result.Permanent {
		patch.CS2CooldownExpiresAt = result.ExpiresAt.UTC().Format(time.RFC3339)
	}
	// syncCooldownTag has already run, so this is the tag list the next full
	// build would produce. Sent with the patch because a cooldown starting or
	// ending is precisely when the list's copy goes stale.
	if tags, err := basic.BuildAccountTagMap(steam.PlatformKey); err == nil {
		patch.Tags = tags[steamID64]
	}
	// No rank fields: the rank is a game stats metric now, refreshed by
	// GameStatsUpdatedEvent rather than carried here.
	s.emitCooldownFn(patch)
}
