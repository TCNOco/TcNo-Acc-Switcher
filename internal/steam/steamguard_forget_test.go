package steam

import (
	"errors"
	"os"
	"testing"

	"TcNo-Acc-Switcher/internal/steam/accountstore"
	steamguardregistry "TcNo-Acc-Switcher/internal/steamguard/registry"
)

// forgetEnv sets a Steam Guard index state for knownIDB and returns the ids the
// registered handler was asked to release.
func forgetEnv(t *testing.T, state steamguardregistry.State) (*SteamService, *[]string) {
	t.Helper()
	env := newSteamTestEnv(t)
	env.writeLoginUsers(t, oneAccountVDF)
	if _, err := accountstore.Upsert(accountstore.Record{
		SteamID64:   knownIDB,
		AccountName: "vault_only",
		Source:      accountstore.SourceSteamGuard,
	}); err != nil {
		t.Fatal(err)
	}
	if state != "" {
		root, err := steamguardregistry.RootPath()
		if err != nil {
			t.Fatal(err)
		}
		// The index refuses to write without its folder, which only a configured
		// vault would have created.
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := steamguardregistry.Upsert(knownIDB, state); err != nil {
			t.Fatal(err)
		}
	}
	released := &[]string{}
	RegisterSteamGuardForgetHandler(func(id string) error {
		*released = append(*released, id)
		return nil
	})
	t.Cleanup(func() { RegisterSteamGuardForgetHandler(nil) })
	return NewSteamService(), released
}

func stillStored(t *testing.T, steamID64 string) bool {
	t.Helper()
	_, ok, err := accountstore.Get(steamID64)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

// The vault holds the only copy of an authenticator's secrets, and the index
// would rebuild the row anyway, so the account cannot leave this way at all.
func TestForgetSteamAccountRefusesAnAuthenticator(t *testing.T) {
	for _, state := range []steamguardregistry.State{
		steamguardregistry.StateActive,
		steamguardregistry.StatePending,
	} {
		t.Run(string(state), func(t *testing.T) {
			svc, released := forgetEnv(t, state)

			err := svc.ForgetSteamAccount(knownIDB)
			stopRefreshTimer(svc)

			if !errors.Is(err, ErrForgetSteamGuardAuthenticator) {
				t.Fatalf("ForgetSteamAccount = %v, want ErrForgetSteamGuardAuthenticator", err)
			}
			if !stillStored(t, knownIDB) {
				t.Fatal("refused forget removed the account anyway")
			}
			if len(*released) != 0 {
				t.Fatalf("released = %v, want nothing deleted from the vault", *released)
			}
		})
	}
}

// A session-only record holds nothing that cannot be signed in for again, so it
// leaves with the account rather than resurrecting its row.
func TestForgetSteamAccountReleasesALoginOnlyRecord(t *testing.T) {
	svc, released := forgetEnv(t, steamguardregistry.StateLoginOnly)

	if err := svc.ForgetSteamAccount(knownIDB); err != nil {
		t.Fatalf("ForgetSteamAccount: %v", err)
	}
	stopRefreshTimer(svc)

	if got := *released; len(got) != 1 || got[0] != knownIDB {
		t.Fatalf("released = %v, want [%s]", got, knownIDB)
	}
	if stillStored(t, knownIDB) {
		t.Fatal("forgotten account still stored")
	}
}

// The record has to go first: forgetting the row while it survives is the state
// this whole path exists to avoid.
func TestForgetSteamAccountAbortsWhenTheRecordCannotGo(t *testing.T) {
	svc, _ := forgetEnv(t, steamguardregistry.StateLoginOnly)
	sentinel := errors.New("vault locked")
	RegisterSteamGuardForgetHandler(func(string) error { return sentinel })

	err := svc.ForgetSteamAccount(knownIDB)
	stopRefreshTimer(svc)

	if !errors.Is(err, sentinel) {
		t.Fatalf("ForgetSteamAccount = %v, want the handler's error", err)
	}
	if !stillStored(t, knownIDB) {
		t.Fatal("account forgotten even though its Steam Guard record remains")
	}
}

// Nothing registered means nothing can delete the record, and forgetting anyway
// is what leaves the nameless row behind.
func TestForgetSteamAccountWithoutAHandlerRefusesALoginOnlyRecord(t *testing.T) {
	svc, _ := forgetEnv(t, steamguardregistry.StateLoginOnly)
	RegisterSteamGuardForgetHandler(nil)

	err := svc.ForgetSteamAccount(knownIDB)
	stopRefreshTimer(svc)

	if !errors.Is(err, ErrForgetSteamGuardUnavailable) {
		t.Fatalf("ForgetSteamAccount = %v, want ErrForgetSteamGuardUnavailable", err)
	}
	if !stillStored(t, knownIDB) {
		t.Fatal("refused forget removed the account anyway")
	}
}

// An account Steam Guard never heard of forgets exactly as it always did.
func TestForgetSteamAccountLeavesSteamGuardAloneWhenItHoldsNothing(t *testing.T) {
	svc, released := forgetEnv(t, "")

	if err := svc.ForgetSteamAccount(knownIDB); err != nil {
		t.Fatalf("ForgetSteamAccount: %v", err)
	}
	stopRefreshTimer(svc)

	if len(*released) != 0 {
		t.Fatalf("released = %v, want nothing", *released)
	}
	if stillStored(t, knownIDB) {
		t.Fatal("forgotten account still stored")
	}
}
