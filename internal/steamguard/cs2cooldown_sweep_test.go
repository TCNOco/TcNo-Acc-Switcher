package steamguard

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/basic"
	"TcNo-Acc-Switcher/internal/steam"
	"TcNo-Acc-Switcher/internal/steamguard/confirmationapi"
	"TcNo-Acc-Switcher/internal/steamguard/cs2cooldown"
)

// fakeCooldownClient stands in for the community transport. Everything except
// FetchCS2GCPD panics: the sweep must not reach any other endpoint.
type fakeCooldownClient struct {
	mu        sync.Mutex
	bodies    map[string][]byte
	errs      map[string]error
	calls     []string
	inFlight  int32
	maxFlight int32
}

func newFakeCooldownClient() *fakeCooldownClient {
	return &fakeCooldownClient{bodies: map[string][]byte{}, errs: map[string]error{}}
}

func (f *fakeCooldownClient) FetchCS2GCPD(ctx context.Context, c confirmationapi.Credentials) ([]byte, error) {
	flight := atomic.AddInt32(&f.inFlight, 1)
	for {
		max := atomic.LoadInt32(&f.maxFlight)
		if flight <= max || atomic.CompareAndSwapInt32(&f.maxFlight, max, flight) {
			break
		}
	}
	defer atomic.AddInt32(&f.inFlight, -1)

	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, c.SteamID)
	if err, ok := f.errs[c.SteamID]; ok {
		return nil, err
	}
	if body, ok := f.bodies[c.SteamID]; ok {
		return body, nil
	}
	return []byte(cleanGCPDPage), nil
}

// The store page is only reached when the Prime setting is on, which the sweep
// tests leave off - so a call here means the sweep spent a request it should
// not have.
func (f *fakeCooldownClient) FetchCS2StorePage(context.Context, confirmationapi.Credentials) ([]byte, error) {
	panic("sweep must not fetch the store page unless Prime collection is enabled")
}

func (f *fakeCooldownClient) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeCooldownClient) List(context.Context, confirmationapi.Credentials) ([]confirmationapi.Confirmation, error) {
	panic("sweep must not list confirmations")
}

func (f *fakeCooldownClient) FetchDetails(context.Context, confirmationapi.Credentials, confirmationapi.Confirmation) (confirmationapi.Details, error) {
	panic("sweep must not fetch details")
}

func (f *fakeCooldownClient) FetchItemClass(context.Context, confirmationapi.Credentials, string, string, string) (confirmationapi.ItemClass, error) {
	panic("sweep must not fetch item classes")
}

func (f *fakeCooldownClient) Decide(context.Context, confirmationapi.Credentials, confirmationapi.Confirmation, confirmationapi.Decision) error {
	panic("sweep must not decide confirmations")
}

func (f *fakeCooldownClient) DecideBatch(context.Context, confirmationapi.Credentials, []confirmationapi.Confirmation, confirmationapi.Decision) error {
	panic("sweep must not decide confirmations")
}

func (f *fakeCooldownClient) CloseIdleConnections() {}

const cleanGCPDPage = `<html><body><h1>Personal Game Data</h1>
<table class="generic_kv_table"><tr><th>Matchmaking Mode</th><th>Wins</th></tr>
<tr><td>Premier</td><td>10</td></tr></table></body></html>`

func cooldownPage(expiry string) string {
	return `<html><body><h1>Personal Game Data</h1>
<table class="generic_kv_table"><tr><th>Cooldown Expiration</th><th>Level</th></tr>
<tr><td>` + expiry + `</td><td>0</td></tr></table></body></html>`
}

// newSweepFixture builds a service whose vault holds one authenticator and one
// login-only record, with the sweep's network faked out.
func newSweepFixture(t *testing.T) (*Service, *fakeCooldownClient, []string) {
	t.Helper()
	service, authID, _ := newAuthServiceFixture(t)
	loginID := seedLoginOnlyRecord(t, service.vault, loginOnlySteamID, "session_only")
	fake := newFakeCooldownClient()
	service.confirmationClient = fake
	// Both records need a live token or the sweep skips them locally.
	setUnexpiredTokens(t, service, authID)
	return service, fake, []string{authID, loginID}
}

// setUnexpiredTokens rewrites the seeded authenticator's access token so the
// local expiry check does not skip it. The fixture ships an opaque placeholder,
// and AccessTokenExpired treats an unreadable token as usable.
func setUnexpiredTokens(t *testing.T, service *Service, accountID string) {
	t.Helper()
	if _, err := recordForSteamID64(service.vault, accountID); err != nil {
		t.Fatal(err)
	}
}

func TestSweepDoesNotRotateTheVaultGeneration(t *testing.T) {
	// The single most important property of the sweep. Any vault write rotates
	// the generation and invalidates every outstanding capability, including the
	// one an open Steam Guard window is holding.
	service, fake, _ := newSweepFixture(t)
	before := service.vault.Generation()

	service.sweepCS2Cooldowns(context.Background())

	if got := service.vault.Generation(); got != before {
		t.Fatalf("generation changed from %q to %q", before, got)
	}
	if fake.callCount() == 0 {
		t.Fatal("sweep made no requests; the fixture is not exercising the path")
	}
}

func TestSweepStoresAndClearsCooldowns(t *testing.T) {
	service, fake, ids := newSweepFixture(t)
	expiry := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Second)
	fake.bodies[ids[0]] = []byte(cooldownPage(expiry.Format("2006-01-02 15:04:05")))
	fake.bodies[ids[1]] = []byte(cleanGCPDPage)

	service.sweepCS2Cooldowns(context.Background())

	entries, err := cs2cooldown.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := entries[ids[0]]; got.CooldownExpiresAt != expiry.Unix() {
		t.Fatalf("cooldown entry = %#v, want expiry %d", got, expiry.Unix())
	}
	// "Parsed, no cooldown" is a positive verdict and must clear, while keeping
	// the entry so CheckedAt keeps driving the per-account floor.
	clean, ok := entries[ids[1]]
	if !ok || clean.CooldownExpiresAt != 0 || clean.CheckedAt == 0 {
		t.Fatalf("clean entry = %#v", clean)
	}
}

func TestSweepLeavesStoredCooldownsAloneOnAnUnreadableResponse(t *testing.T) {
	service, fake, ids := newSweepFixture(t)
	seeded := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	checkedAt := time.Now().Add(-24 * time.Hour)
	if err := cs2cooldown.Put(ids[0], seeded, false, checkedAt); err != nil {
		t.Fatal(err)
	}
	// A sign-in page and an unrecognised body must both change nothing: clearing
	// on a page we could not read would delete a real cooldown.
	fake.bodies[ids[0]] = []byte(`<html><body>Personal Game Data<script>g_steamID = false;</script></body></html>`)
	fake.bodies[ids[1]] = []byte(`<html><body>something else entirely</body></html>`)

	service.sweepCS2Cooldowns(context.Background())

	entries, err := cs2cooldown.Load()
	if err != nil {
		t.Fatal(err)
	}
	got := entries[ids[0]]
	if got.CooldownExpiresAt != seeded.Unix() || got.CheckedAt != checkedAt.Unix() {
		t.Fatalf("entry = %#v, want it untouched including CheckedAt", got)
	}
	if _, ok := entries[ids[1]]; ok {
		t.Fatal("an unrecognised response created an entry")
	}
}

func TestSweepTagsAndUntagsAccountsWithTheCooldown(t *testing.T) {
	service, fake, ids := newSweepFixture(t)
	expiry := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Second)
	fake.bodies[ids[0]] = []byte(cooldownPage(expiry.Format("2006-01-02 15:04:05")))
	fake.bodies[ids[1]] = []byte(cleanGCPDPage)

	service.sweepCS2Cooldowns(context.Background())

	tagged, err := basic.BuildAccountTagMap(steam.PlatformKey)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCooldownTag(tagged[ids[0]]) {
		t.Fatalf("account on cooldown was not tagged: %#v", tagged[ids[0]])
	}
	if hasCooldownTag(tagged[ids[1]]) {
		t.Fatalf("clean account was tagged: %#v", tagged[ids[1]])
	}

	// The cooldown lifting must take the tag with it.
	fake.bodies[ids[0]] = []byte(cleanGCPDPage)
	service.cooldownSweep.mu.Lock()
	service.cooldownSweep.lastSweep = time.Time{}
	service.cooldownSweep.mu.Unlock()
	if err := cs2cooldown.Clear(ids[0], time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	service.sweepCS2Cooldowns(context.Background())

	tagged, err = basic.BuildAccountTagMap(steam.PlatformKey)
	if err != nil {
		t.Fatal(err)
	}
	if hasCooldownTag(tagged[ids[0]]) {
		t.Fatalf("tag survived the cooldown ending: %#v", tagged[ids[0]])
	}
}

func TestSweepLeavesTheTagAloneOnAnUnreadableResponse(t *testing.T) {
	// One lapsed session must not untag every account on the list.
	service, fake, ids := newSweepFixture(t)
	if err := basic.SetManagedTag(steam.PlatformKey, ids[0], basic.ManagedTagCS2Cooldown, true, ""); err != nil {
		t.Fatal(err)
	}
	fake.bodies[ids[0]] = []byte(`<html><body>Personal Game Data<script>g_steamID = false;</script></body></html>`)

	service.sweepCS2Cooldowns(context.Background())

	tagged, err := basic.BuildAccountTagMap(steam.PlatformKey)
	if err != nil {
		t.Fatal(err)
	}
	if !hasCooldownTag(tagged[ids[0]]) {
		t.Fatalf("tag was removed on an unreadable page: %#v", tagged[ids[0]])
	}
}

func hasCooldownTag(tags []basic.AccountTagDTO) bool {
	for _, tag := range tags {
		if tag.Name == basic.ManagedTagCS2Cooldown {
			return true
		}
	}
	return false
}

func TestSweepAbortsTheWholeRunOnARateLimit(t *testing.T) {
	// Walking the rest of the list into the same wall is what earns a longer ban.
	service, fake, ids := newSweepFixture(t)
	fake.errs[ids[0]] = &confirmationapi.Error{Kind: confirmationapi.FailureRateLimit}

	service.sweepCS2Cooldowns(context.Background())

	if got := fake.callCount(); got != 1 {
		t.Fatalf("made %d requests after a rate limit, want 1", got)
	}
}

func TestSweepSkipsAccountsCheckedRecently(t *testing.T) {
	service, fake, ids := newSweepFixture(t)
	for _, id := range ids {
		if err := cs2cooldown.Clear(id, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	service.sweepCS2Cooldowns(context.Background())
	if got := fake.callCount(); got != 0 {
		t.Fatalf("made %d requests inside the per-account floor, want 0", got)
	}
}

func TestSweepIsSerial(t *testing.T) {
	service, fake, _ := newSweepFixture(t)
	service.sweepCS2Cooldowns(context.Background())
	if got := atomic.LoadInt32(&fake.maxFlight); got > 1 {
		t.Fatalf("max concurrent requests = %d, want 1", got)
	}
}

func TestSweepStopsPromptlyWhenCancelled(t *testing.T) {
	service, fake, _ := newSweepFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service.sweepCS2Cooldowns(ctx)
	if got := fake.callCount(); got != 0 {
		t.Fatalf("made %d requests after cancellation, want 0", got)
	}
}

// setCS2Settings applies the three CS2 toggles and restores them afterwards.
//
// paths.ResetForTest does not reset settingsDirOnce, so SteamSettings.json is
// shared by every test in this process; without the restore these would leak
// into every test that runs after.
func setCS2Settings(t *testing.T, cooldown, rank, prime bool) {
	t.Helper()
	settings, err := steam.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	before := [3]bool{
		settings.SteamCollectCS2Cooldowns,
		settings.SteamShowCS2Rank,
		settings.SteamShowCS2PrimeTag,
	}
	t.Cleanup(func() {
		restore, err := steam.LoadSettings()
		if err != nil {
			return
		}
		restore.SteamCollectCS2Cooldowns = before[0]
		restore.SteamShowCS2Rank = before[1]
		restore.SteamShowCS2PrimeTag = before[2]
		_ = steam.SaveSettings(restore)
	})
	settings.SteamCollectCS2Cooldowns = cooldown
	settings.SteamShowCS2Rank = rank
	settings.SteamShowCS2PrimeTag = prime
	if err := steam.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
}

func TestSweepRespectsTheSteamSetting(t *testing.T) {
	service, fake, _ := newSweepFixture(t)
	setCS2Settings(t, false, false, false)
	service.sweepCS2Cooldowns(context.Background())
	if got := fake.callCount(); got != 0 {
		t.Fatalf("made %d requests with every CS2 state disabled, want 0", got)
	}
}

// The three settings are independent: the rank and Prime states are read from
// the same response, so either one alone is reason enough to fetch it.
func TestSweepRunsForRankAloneWithCooldownOff(t *testing.T) {
	service, fake, _ := newSweepFixture(t)
	setCS2Settings(t, false, true, false)
	service.sweepCS2Cooldowns(context.Background())
	if got := fake.callCount(); got == 0 {
		t.Fatal("made no requests with only the rank state enabled, want at least one")
	}
}

func TestSweepEmitsAPatchPerUpdatedAccount(t *testing.T) {
	service, fake, ids := newSweepFixture(t)
	expiry := time.Now().Add(3 * time.Hour).UTC().Truncate(time.Second)
	fake.bodies[ids[0]] = []byte(cooldownPage(expiry.Format("2006-01-02 15:04:05")))

	var mu sync.Mutex
	var patches []steam.CS2CooldownPatch
	service.emitCooldownFn = func(p steam.CS2CooldownPatch) {
		mu.Lock()
		defer mu.Unlock()
		patches = append(patches, p)
	}

	service.sweepCS2Cooldowns(context.Background())

	mu.Lock()
	defer mu.Unlock()
	found := false
	for _, patch := range patches {
		if patch.SteamID64 == ids[0] && patch.CS2CooldownExpiresAt == expiry.Format(time.RFC3339) {
			found = true
		}
	}
	if !found {
		t.Fatalf("patches = %#v, want one for %s at %s", patches, ids[0], expiry.Format(time.RFC3339))
	}
}

func TestSignalCooldownSweepDoesNotBlock(t *testing.T) {
	// It is called from inside the unlock path with s.mu held; blocking there
	// would deadlock the whole service.
	service := newServiceForTest()
	for i := 0; i < 10; i++ {
		service.signalCooldownSweep()
	}
	if len(service.cooldownSweep.wake) != 1 {
		t.Fatalf("wake channel holds %d signals, want 1", len(service.cooldownSweep.wake))
	}
}

func TestSweepSkipsWhenAnotherIsRunning(t *testing.T) {
	service, fake, _ := newSweepFixture(t)
	service.cooldownSweep.mu.Lock()
	service.cooldownSweep.running = true
	service.cooldownSweep.mu.Unlock()

	service.sweepCS2Cooldowns(context.Background())
	if got := fake.callCount(); got != 0 {
		t.Fatalf("made %d requests while another sweep was running, want 0", got)
	}
}
