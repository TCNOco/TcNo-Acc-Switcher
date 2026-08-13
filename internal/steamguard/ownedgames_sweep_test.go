package steamguard

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/steam"
	"TcNo-Acc-Switcher/internal/steam/ownedgames"
	"TcNo-Acc-Switcher/internal/steamguard/confirmationapi"
	"TcNo-Acc-Switcher/internal/steamguard/protocol"
	"TcNo-Acc-Switcher/internal/steamguard/sessionrefresh"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

// fakeOwnedGamesClient stands in for the Web API transport. Everything except
// FetchOwnedApps panics: the sweep must not reach any other endpoint.
type fakeOwnedGamesClient struct {
	mu    sync.Mutex
	apps  map[string][]uint32
	errs  map[string]error
	calls []string
	// afterFetch runs once each request has been answered, so a test can change
	// the world - offline mode, say - between two accounts of one sweep.
	afterFetch func()
}

func newFakeOwnedGamesClient() *fakeOwnedGamesClient {
	return &fakeOwnedGamesClient{apps: map[string][]uint32{}, errs: map[string]error{}}
}

func (f *fakeOwnedGamesClient) FetchOwnedApps(_ context.Context, c confirmationapi.Credentials) ([]uint32, error) {
	f.mu.Lock()
	f.calls = append(f.calls, c.SteamID)
	err, failed := f.errs[c.SteamID]
	apps, known := f.apps[c.SteamID]
	afterFetch := f.afterFetch
	f.mu.Unlock()
	if afterFetch != nil {
		afterFetch()
	}
	if failed {
		return nil, err
	}
	if known {
		return apps, nil
	}
	return []uint32{730}, nil
}

func (f *fakeOwnedGamesClient) callsFor(steamID64 string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, id := range f.calls {
		if id == steamID64 {
			count++
		}
	}
	return count
}

func (f *fakeOwnedGamesClient) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeOwnedGamesClient) FetchCS2GCPD(context.Context, confirmationapi.Credentials) ([]byte, error) {
	panic("owned games sweep must not fetch the GCPD page")
}

func (f *fakeOwnedGamesClient) FetchCS2StorePage(context.Context, confirmationapi.Credentials) ([]byte, error) {
	panic("owned games sweep must not fetch the store page")
}

func (f *fakeOwnedGamesClient) FetchTradeOfferPrivacyPage(context.Context, confirmationapi.Credentials) ([]byte, error) {
	panic("owned games sweep must not fetch the trade offer privacy page")
}

func (f *fakeOwnedGamesClient) List(context.Context, confirmationapi.Credentials) ([]confirmationapi.Confirmation, error) {
	panic("owned games sweep must not list confirmations")
}

func (f *fakeOwnedGamesClient) FetchDetails(context.Context, confirmationapi.Credentials, confirmationapi.Confirmation) (confirmationapi.Details, error) {
	panic("owned games sweep must not fetch details")
}

func (f *fakeOwnedGamesClient) FetchItemClass(context.Context, confirmationapi.Credentials, string, string, string) (confirmationapi.ItemClass, error) {
	panic("owned games sweep must not fetch item classes")
}

func (f *fakeOwnedGamesClient) Decide(context.Context, confirmationapi.Credentials, confirmationapi.Confirmation, confirmationapi.Decision) error {
	panic("owned games sweep must not decide confirmations")
}

func (f *fakeOwnedGamesClient) DecideBatch(context.Context, confirmationapi.Credentials, []confirmationapi.Confirmation, confirmationapi.Decision) error {
	panic("owned games sweep must not decide confirmations")
}

func (f *fakeOwnedGamesClient) CloseIdleConnections() {}

// batchRefresher records every renewal the sweep asks for. Refresh panics: one
// call per account is exactly the cost RefreshBatch exists to avoid.
type batchRefresher struct {
	inner   steamSessionRefresher
	mu      sync.Mutex
	batches [][]uint64
}

func (r *batchRefresher) Refresh(context.Context, uint64) (sessionrefresh.Result, error) {
	panic("owned games sweep must renew in one batch, not one account at a time")
}

func (r *batchRefresher) RefreshBatch(ctx context.Context, steamIDs []uint64) ([]sessionrefresh.Result, error) {
	r.mu.Lock()
	r.batches = append(r.batches, append([]uint64(nil), steamIDs...))
	r.mu.Unlock()
	return r.inner.RefreshBatch(ctx, steamIDs)
}

func (r *batchRefresher) recorded() [][]uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.batches
}

// newOwnedGamesFixture builds a service whose vault holds one authenticator and
// one login-only record, both with a live session, and fakes out the network.
// The seeded tokens are opaque, and AccessTokenExpired treats a token it cannot
// read as usable.
func newOwnedGamesFixture(t *testing.T) (*Service, *fakeOwnedGamesClient, []string) {
	t.Helper()
	service, authID, _ := newAuthServiceFixture(t)
	loginID := seedLoginOnlyRecord(t, service.vault, loginOnlySteamID, "session_only")
	fake := newFakeOwnedGamesClient()
	service.confirmationClient = fake
	return service, fake, []string{authID, loginID}
}

func TestOwnedGamesSweepStoresLibraries(t *testing.T) {
	service, fake, ids := newOwnedGamesFixture(t)
	fake.apps[ids[0]] = []uint32{440, 730, 440}
	fake.apps[ids[1]] = []uint32{570}

	service.sweepOwnedGames(context.Background())

	entries, err := ownedgames.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := entries[ids[0]].AppIDs; len(got) != 2 || got[0] != 440 || got[1] != 730 {
		t.Fatalf("stored app ids = %v, want [440 730]", got)
	}
	if entries[ids[0]].CheckedAt == 0 {
		t.Fatal("stored entry has no CheckedAt, so the per-account floor cannot work")
	}
	if got := entries[ids[1]].AppIDs; len(got) != 1 || got[0] != 570 {
		t.Fatalf("stored app ids = %v, want [570]", got)
	}
}

func TestOwnedGamesSweepStoresNothingForARefusedToken(t *testing.T) {
	// Steam answers a caller it will not authorise with an empty library rather
	// than an error. Storing that would leave the account permanently blank.
	service, fake, ids := newOwnedGamesFixture(t)
	if err := ownedgames.Put(ids[0], []uint32{440}, time.Now().Add(-24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	fake.errs[ids[0]] = &confirmationapi.Error{Kind: confirmationapi.FailureReauth}

	service.sweepOwnedGames(context.Background())

	entries, err := ownedgames.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := entries[ids[0]].AppIDs; len(got) != 1 || got[0] != 440 {
		t.Fatalf("refused account = %v, want the previously stored [440] untouched", got)
	}
	if _, ok := entries[ids[1]]; !ok {
		t.Fatal("one refused account stopped the rest of the sweep")
	}
}

func TestOwnedGamesSweepRenewsEveryLapsedAccountInOneBatch(t *testing.T) {
	// One RefreshBatch is the whole reason the sweep may write to the vault at
	// all: each generation switch invalidates every outstanding capability, so N
	// single refreshes would cost N invalidations.
	service, fake, ids := newOwnedGamesFixture(t)
	lapsed := accessTokenExpiringAt(time.Now().Add(-time.Hour))
	lapsedA := seedLoginOnlyRecordWithTokens(
		t, service.vault, loginOnlySteamID+1, "lapsed_a", lapsed, "refresh-token-for-tests")
	lapsedB := seedLoginOnlyRecordWithTokens(
		t, service.vault, loginOnlySteamID+2, "lapsed_b", lapsed, "refresh-token-for-tests")

	client := &authServiceTokenClient{result: protocol.TokenResult{
		State:        protocol.AuthResultTokenIssued,
		AccessToken:  accessTokenExpiringAt(time.Now().Add(time.Hour)),
		RefreshToken: "renewed-refresh-token",
	}}
	var refresher *batchRefresher
	service.newSessionRefresher = func(v *vault.Vault) steamSessionRefresher {
		refresher = &batchRefresher{inner: sessionrefresh.New(client, v)}
		return refresher
	}

	service.sweepOwnedGames(context.Background())

	if refresher == nil {
		t.Fatal("no renewal was attempted for two lapsed accounts")
	}
	batches := refresher.recorded()
	if len(batches) != 1 {
		t.Fatalf("renewal batches = %v, want exactly one", batches)
	}
	want := map[uint64]bool{loginOnlySteamID + 1: true, loginOnlySteamID + 2: true}
	if len(batches[0]) != len(want) {
		t.Fatalf("batch = %v, want the two lapsed accounts only", batches[0])
	}
	for _, id := range batches[0] {
		if !want[id] {
			t.Fatalf("batch = %v, want the two lapsed accounts only", batches[0])
		}
	}
	// The renewed tokens are only usable if the sweep re-read them out of the
	// vault after the batch wrote it.
	for _, id := range []string{lapsedA, lapsedB} {
		if got := fake.callsFor(id); got != 1 {
			t.Fatalf("renewed account %s made %d requests, want 1", id, got)
		}
	}
	if fake.callsFor(ids[0]) != 1 || fake.callsFor(ids[1]) != 1 {
		t.Fatal("the live accounts were not swept alongside the renewed ones")
	}
}

func TestOwnedGamesSweepSkipsALapsedAccountItCannotRenew(t *testing.T) {
	service, fake, _ := newOwnedGamesFixture(t)
	seedLoginOnlyRecordWithTokens(t, service.vault, loginOnlySteamID+3, "no_refresh",
		accessTokenExpiringAt(time.Now().Add(-time.Hour)), "")
	service.newSessionRefresher = func(*vault.Vault) steamSessionRefresher {
		panic("there is nothing to renew from, so Steam must not be contacted")
	}

	service.sweepOwnedGames(context.Background())

	if got := fake.callsFor(strconv.FormatUint(loginOnlySteamID+3, 10)); got != 0 {
		t.Fatalf("made %d requests for an account with no usable session, want 0", got)
	}
}

func TestOwnedGamesSweepSkipsAccountsCheckedRecently(t *testing.T) {
	service, fake, ids := newOwnedGamesFixture(t)
	for _, id := range ids {
		if err := ownedgames.Put(id, []uint32{730}, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	service.sweepOwnedGames(context.Background())
	if got := fake.callCount(); got != 0 {
		t.Fatalf("made %d requests inside the per-account floor, want 0", got)
	}
}

func TestOwnedGamesSweepEmitsPerAccount(t *testing.T) {
	service, fake, ids := newOwnedGamesFixture(t)
	fake.apps[ids[0]] = []uint32{440, 730}

	var mu sync.Mutex
	patches := map[string]int{}
	service.emitOwnedGamesFn = func(p steam.OwnedGamesPatch) {
		mu.Lock()
		defer mu.Unlock()
		patches[p.SteamID64] = p.AppCount
	}

	service.sweepOwnedGames(context.Background())

	mu.Lock()
	defer mu.Unlock()
	if patches[ids[0]] != 2 {
		t.Fatalf("patches = %#v, want %s with 2 apps", patches, ids[0])
	}
}

func TestOwnedGamesSweepIsANoOpWhenItMayNotRun(t *testing.T) {
	cases := []struct {
		name  string
		apply func(t *testing.T, service *Service)
	}{
		{
			name: "offline",
			apply: func(t *testing.T, _ *Service) {
				appclient.SetOfflineMode(true)
				t.Cleanup(func() { appclient.SetOfflineMode(false) })
			},
		},
		{
			name: "app locked",
			apply: func(t *testing.T, _ *Service) {
				const appPassword = "correct horse battery staple"
				if err := security.SetAppPassword(appPassword); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					_ = security.UnlockApp(appPassword)
					_ = security.RemoveAppPassword(appPassword)
				})
				if err := security.Lock(); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "vault locked",
			apply: func(t *testing.T, service *Service) {
				if err := service.vault.Lock(); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "feature disabled",
			apply: func(t *testing.T, _ *Service) {
				settings, err := LoadSettings()
				if err != nil {
					t.Fatal(err)
				}
				settings.FeatureEnabled = false
				if err := SaveSettings(settings); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:  "cancelled",
			apply: func(_ *testing.T, _ *Service) {},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service, fake, _ := newOwnedGamesFixture(t)
			testCase.apply(t, service)
			ctx, cancel := context.WithCancel(context.Background())
			if testCase.name == "cancelled" {
				cancel()
			}
			defer cancel()

			service.sweepOwnedGames(ctx)

			if got := fake.callCount(); got != 0 {
				t.Fatalf("made %d requests while %s, want 0", got, testCase.name)
			}
		})
	}
}

func TestOwnedGamesSweepStopsWhenOfflineSwitchesOnMidSweep(t *testing.T) {
	// Offline mode is a promise the app makes for as long as it is on, not only
	// at the moment a sweep starts.
	service, fake, _ := newOwnedGamesFixture(t)
	t.Cleanup(func() { appclient.SetOfflineMode(false) })
	fake.afterFetch = func() { appclient.SetOfflineMode(true) }

	service.sweepOwnedGames(context.Background())

	if got := fake.callCount(); got != 1 {
		t.Fatalf("made %d requests, want 1: the sweep continued after offline mode was switched on", got)
	}
}

func TestOwnedGamesSweepRenewsNoSessionsWhileOffline(t *testing.T) {
	// A renewal is a Steam token call per account, and it runs between the
	// sweep's entry check and its per-account loop.
	service, _, _ := newOwnedGamesFixture(t)
	appclient.SetOfflineMode(true)
	t.Cleanup(func() { appclient.SetOfflineMode(false) })
	service.newSessionRefresher = func(*vault.Vault) steamSessionRefresher {
		panic("offline mode must not reach a Steam token call")
	}

	service.refreshLapsedOwnedGamesSessions(context.Background(), []uint64{loginOnlySteamID})
}

func TestOwnedGamesSweepIntervalFollowsTheVaultLease(t *testing.T) {
	service, _, _ := newOwnedGamesFixture(t)
	if err := service.vault.SetLeaseMode(vault.FixedLease); err != nil {
		t.Fatal(err)
	}
	if got := service.ownedGamesSweepIntervalNow(); got != ownedGamesSweepInterval {
		t.Fatalf("interval under a fixed lease = %s, want %s", got, ownedGamesSweepInterval)
	}
	if err := service.vault.SetLeaseMode(vault.ProcessLease); err != nil {
		t.Fatal(err)
	}
	if got := service.ownedGamesSweepIntervalNow(); got != ownedGamesRememberedSweepInterval {
		t.Fatalf("interval under a remembered password = %s, want %s", got, ownedGamesRememberedSweepInterval)
	}
	if err := service.vault.Lock(); err != nil {
		t.Fatal(err)
	}
	if got := service.ownedGamesSweepIntervalNow(); got != ownedGamesSweepInterval {
		t.Fatalf("interval with a locked vault = %s, want %s", got, ownedGamesSweepInterval)
	}
}

func TestOwnedGamesSweepDueFollowsALeaseChangedMidSession(t *testing.T) {
	// Remember-password can be switched on long after the sweeper goroutine
	// started, and the shorter interval has to reach the sweeper already waiting
	// out the longer one.
	service, _, _ := newOwnedGamesFixture(t)
	if err := service.vault.SetLeaseMode(vault.FixedLease); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	service.ownedGamesSweep.mu.Lock()
	service.ownedGamesSweep.lastSweep = now.Add(-ownedGamesRememberedSweepInterval - time.Minute)
	service.ownedGamesSweep.mu.Unlock()

	if service.ownedGamesSweepDue(now) {
		t.Fatal("a sweep came due inside the 24h interval a fixed lease calls for")
	}
	if err := service.vault.SetLeaseMode(vault.ProcessLease); err != nil {
		t.Fatal(err)
	}
	if !service.ownedGamesSweepDue(now) {
		t.Fatal("remembering the password mid-session did not shorten the interval")
	}
}

func TestSignalOwnedGamesSweepDoesNotBlock(t *testing.T) {
	// It is called from inside the unlock path with s.mu held; blocking there
	// would deadlock the whole service.
	service := newServiceForTest()
	for i := 0; i < 10; i++ {
		service.signalOwnedGamesSweep()
	}
	if len(service.ownedGamesSweep.wake) != 1 {
		t.Fatalf("wake channel holds %d signals, want 1", len(service.ownedGamesSweep.wake))
	}
}
