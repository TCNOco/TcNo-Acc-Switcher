package steambrowser

import (
	"strings"
	"testing"
)

func TestSiteDestination(t *testing.T) {
	store, err := SiteStore.Destination(testSteamID)
	if err != nil {
		t.Fatalf("store destination: %v", err)
	}
	if !IsTrusted(PlatformSteam, store) {
		t.Errorf("store destination %q is not trusted", store)
	}

	community, err := SiteCommunity.Destination(testSteamID)
	if err != nil {
		t.Fatalf("community destination: %v", err)
	}
	if !IsTrusted(PlatformSteam, community) {
		t.Errorf("community destination %q is not trusted", community)
	}
	// Community lands on the account's own profile, which is the page the user
	// asked for by picking that account.
	if !strings.Contains(community, testSteamID) {
		t.Errorf("community destination %q does not name the account", community)
	}

	if _, err := SiteCommunity.Destination("not-a-steam-id"); err == nil {
		t.Error("a bad Steam ID produced a community destination")
	}
	if _, err := Site("chat").Destination(testSteamID); err == nil {
		t.Error("an unknown site produced a destination")
	}
}

func TestRegistryEnforcesTheWindowCap(t *testing.T) {
	r := newRegistry()
	for i := 0; i < MaxWindows; i++ {
		if _, err := r.reserve(); err != nil {
			t.Fatalf("reserve %d: %v", i, err)
		}
	}
	if _, err := r.reserve(); err != ErrTooManyWindows {
		t.Errorf("reserve past the cap: %v, want ErrTooManyWindows", err)
	}
}

// A reservation that never becomes a window must not leak a slot, or repeated
// failures would wedge the feature until restart.
func TestRegistryReleaseFreesTheSlot(t *testing.T) {
	r := newRegistry()
	ids := make([]string, 0, MaxWindows)
	for i := 0; i < MaxWindows; i++ {
		id, err := r.reserve()
		if err != nil {
			t.Fatalf("reserve %d: %v", i, err)
		}
		ids = append(ids, id)
	}
	r.release(ids[0])
	if _, err := r.reserve(); err != nil {
		t.Errorf("reserve after release: %v, want success", err)
	}
}

func TestRegistryReservationIsNotYetAWindow(t *testing.T) {
	r := newRegistry()
	id, err := r.reserve()
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	// Commands must not find a window that is still being built.
	if _, err := r.get(id); err != ErrNoSuchWindow {
		t.Errorf("get on a reservation: %v, want ErrNoSuchWindow", err)
	}
	r.add(&session{id: id, steamID64: testSteamID})
	if _, err := r.get(id); err != nil {
		t.Errorf("get after add: %v", err)
	}
}

func TestRegistryRemoveIsIdempotent(t *testing.T) {
	r := newRegistry()
	id, err := r.reserve()
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	r.add(&session{id: id})
	if got := r.remove(id); got == nil {
		t.Fatal("first remove returned nothing")
	}
	// A second close of the same window must not hand back the view again, or it
	// would be released twice.
	if got := r.remove(id); got != nil {
		t.Error("second remove returned a session")
	}
	if r.count() != 0 {
		t.Errorf("count = %d, want 0", r.count())
	}
}

func TestRegistryIdsSkipsReservations(t *testing.T) {
	r := newRegistry()
	open, err := r.reserve()
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	r.add(&session{id: open})
	if _, err := r.reserve(); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	ids := r.ids()
	if len(ids) != 1 || ids[0] != open {
		t.Errorf("ids() = %v, want just the opened window %q", ids, open)
	}
}

func TestSessionKeysAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 32; i++ {
		key, err := newSessionKey()
		if err != nil {
			t.Fatalf("newSessionKey: %v", err)
		}
		if seen[key] {
			t.Fatalf("duplicate window id %q", key)
		}
		seen[key] = true
	}
}
