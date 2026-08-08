package steamguard

import (
	"testing"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/steam/accountstore"
	"TcNo-Acc-Switcher/internal/steamguard/registry"
)

const seedSteamID = "76561198000000100"

func newRegistrySeedService(t *testing.T) *Service {
	t.Helper()
	paths.ResetForTest(t.TempDir())
	return &Service{registryUpsertFn: func(string, registry.State) error { return nil }}
}

// Every enrollment route reaches the registration index through upsertRegistry.
// Seeding there is what stops the two indexes disagreeing about which accounts
// exist - a vault record the Steam list has never heard of would show no tile.
func TestUpsertRegistrySeedsTheAccountStore(t *testing.T) {
	s := newRegistrySeedService(t)

	if !s.upsertRegistry(seedSteamID, registry.StatePending) {
		t.Fatal("upsertRegistry reported failure")
	}

	rec, ok, err := accountstore.Get(seedSteamID)
	if err != nil {
		t.Fatalf("accountstore.Get: %v", err)
	}
	if !ok {
		t.Fatal("registry write left no account store record")
	}
	if rec.SteamID64 != seedSteamID {
		t.Errorf("stored %q, want %q", rec.SteamID64, seedSteamID)
	}
}

// upsertRegistry has only the ID. It must not blank a login name a named path
// (login-only enrollment, maFile import) already recorded.
func TestUpsertRegistryDoesNotClobberAStoredName(t *testing.T) {
	s := newRegistrySeedService(t)
	s.rememberSteamAccount(seedSteamID, "vault_only")

	if !s.upsertRegistry(seedSteamID, registry.StateActive) {
		t.Fatal("upsertRegistry reported failure")
	}

	rec, _, err := accountstore.Get(seedSteamID)
	if err != nil {
		t.Fatalf("accountstore.Get: %v", err)
	}
	if rec.AccountName != "vault_only" {
		t.Errorf("AccountName = %q, want it preserved", rec.AccountName)
	}
}

// A failed registry write must not leave a store record behind, or the list
// would show an account the vault never actually took.
func TestUpsertRegistryFailureLeavesTheStoreUntouched(t *testing.T) {
	paths.ResetForTest(t.TempDir())
	s := &Service{registryUpsertFn: func(string, registry.State) error { return registry.ErrInvalidIndex }}

	if s.upsertRegistry(seedSteamID, registry.StateActive) {
		t.Fatal("upsertRegistry should report failure")
	}
	if _, ok, err := accountstore.Get(seedSteamID); err != nil || ok {
		t.Fatalf("store record created despite a failed registry write: ok=%v err=%v", ok, err)
	}
}
