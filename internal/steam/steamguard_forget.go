package steam

import (
	"errors"
	"log/slog"
	"strings"
	"sync"

	steamguardregistry "TcNo-Acc-Switcher/internal/steamguard/registry"
)

// Refusals the Forget flow can raise. Their messages are i18n keys, the way the
// rest of this package's user-facing errors are.
var (
	// ErrForgetSteamGuardAuthenticator refuses to forget an account whose vault
	// record holds an authenticator, half-finished or complete.
	//
	// Forgetting drops the switcher's copy of an account, but the vault record
	// outlives it and the account list rebuilds a row from the Steam Guard index,
	// so the account comes back without the login name Steam cannot sign in
	// without. Deleting the record instead is no answer either: an authenticator's
	// shared secret and its revocation code exist nowhere else, and nothing here
	// can put them back. So this path refuses rather than doing half of either.
	ErrForgetSteamGuardAuthenticator = errors.New("Toast_Steam_ForgetHasAuthenticator")

	// ErrForgetSteamGuardLocked reports that a session-only record cannot be
	// deleted until the vault is open, which is what the deletion needs its keys
	// for.
	ErrForgetSteamGuardLocked = errors.New("Toast_Steam_ForgetNeedsVaultUnlock")

	// ErrForgetSteamGuardUnavailable reports that nothing is wired up to delete
	// the record. Forgetting anyway would leave the row to return nameless.
	ErrForgetSteamGuardUnavailable = errors.New("Toast_Steam_ForgetGuardUnavailable")
)

var (
	steamGuardForgetMu sync.Mutex
	steamGuardForgetFn func(steamID64 string) error
)

// RegisterSteamGuardForgetHandler supplies the way to delete a session-only Steam
// Guard record for an account the switcher is forgetting. Reading and writing the
// vault is the Steam Guard package's job, so it registers this at startup.
func RegisterSteamGuardForgetHandler(fn func(steamID64 string) error) {
	steamGuardForgetMu.Lock()
	defer steamGuardForgetMu.Unlock()
	steamGuardForgetFn = fn
}

func steamGuardForgetHandler() func(steamID64 string) error {
	steamGuardForgetMu.Lock()
	defer steamGuardForgetMu.Unlock()
	return steamGuardForgetFn
}

// releaseSteamGuardRecord clears the Steam Guard side of an account that is about
// to be forgotten, and decides whether the forget may go ahead at all.
//
// Nothing in Steam Guard: nothing to do. A session-only record: it goes with the
// account, since it holds no secret the user cannot simply sign in for again. An
// authenticator: refused, so the caller stops before anything has been removed.
func releaseSteamGuardRecord(steamID64 string) error {
	steamID64 = strings.TrimSpace(steamID64)
	entries, err := steamguardregistry.Load()
	if err != nil {
		// An index this build cannot read is also one the account list cannot
		// rebuild a row from, so forgetting is no worse off for going ahead -
		// and there is nothing here to tell it what to delete.
		steamLog.Warn("Steam Guard account state unavailable while forgetting",
			slog.String("steamId", tailSteamID(steamID64)), slog.Any("err", err))
		return nil
	}

	var state steamguardregistry.State
	found := false
	for _, entry := range entries {
		if strings.TrimSpace(entry.SteamID64) != steamID64 {
			continue
		}
		state, found = entry.State, true
		break
	}
	if !found {
		return nil
	}
	if state != steamguardregistry.StateLoginOnly {
		return ErrForgetSteamGuardAuthenticator
	}
	forget := steamGuardForgetHandler()
	if forget == nil {
		return ErrForgetSteamGuardUnavailable
	}
	return forget(steamID64)
}
