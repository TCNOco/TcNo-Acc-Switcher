package steam

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"TcNo-Acc-Switcher/internal/steam/accountstore"
	steamguardregistry "TcNo-Acc-Switcher/internal/steamguard/registry"
)

var (
	accountNameResolverMu sync.Mutex
	accountNameResolver   func(steamID64 string) (string, bool)
)

// RegisterAccountNameResolver supplies a way to look up an account's Steam login
// name when the switcher knows only its SteamID64.
//
// The account store learns names from loginusers.vdf, so an account that only
// ever existed in a restored Steam Guard vault has none - and the login name is
// the one thing Steam cannot sign in without, because AutoLoginUser is it. The
// vault does hold it, but reading the vault is the Steam Guard package's job, so
// it registers this at startup. Unregistered, and for a locked vault, callers
// simply get no name.
func RegisterAccountNameResolver(fn func(steamID64 string) (string, bool)) {
	accountNameResolverMu.Lock()
	defer accountNameResolverMu.Unlock()
	accountNameResolver = fn
}

func resolveAccountName(steamID64 string) (string, bool) {
	accountNameResolverMu.Lock()
	fn := accountNameResolver
	accountNameResolverMu.Unlock()
	if fn == nil {
		return "", false
	}
	name, ok := fn(steamID64)
	name = strings.TrimSpace(name)
	return name, ok && name != ""
}

// knownAccountsForRoot is the account list every caller should use: Steam's
// current rows plus the ones only the switcher remembers. A loginusers.vdf that
// is missing or corrupt costs the accounts Steam still knew about, not all of
// them, so it is a warning rather than a failure.
func knownAccountsForRoot(steamRoot string) []LoginUser {
	loginPath := LoginUsersPath(steamRoot)
	users, err := ParseLoginUsers(loginPath)
	if err != nil {
		steamLog.Warn("ParseLoginUsers failed; falling back to the account store",
			slog.String("path", loginPath), slog.Any("err", err))
		users = nil
	}
	return syncKnownAccounts(users)
}

// syncKnownAccounts folds the current loginusers.vdf rows into the persistent
// account store and returns what the switcher should show: every vdf row in
// Steam's own order, then every stored account Steam no longer knows about.
//
// This is the seam that decouples the account list from Steam's file. Both the
// list and the background profile refresh go through it, so a truncated
// loginusers.vdf costs the user nothing.
func syncKnownAccounts(users []LoginUser) []LoginUser {
	stored, err := accountstore.Load()
	if err != nil {
		// A store we cannot parse must neither blank the list nor be written
		// over - it is the only copy of accounts Steam has already forgotten.
		steamLog.Warn("Steam account store unavailable", slog.Any("err", err))
		return users
	}

	known := make(map[string]struct{}, len(stored))
	for _, rec := range stored {
		known[rec.SteamID64] = struct{}{}
	}

	incoming := recordsFromLoginUsers(users)
	incoming = append(incoming, recordsFromSteamGuardRegistry()...)
	if changed, err := accountstore.UpsertMany(incoming); err != nil {
		steamLog.Warn("could not update the Steam account store", slog.Any("err", err))
	} else if changed {
		// The upsert can add accounts the load above could not see - a Steam
		// Guard folder swapped in behind the app's back is exactly that - and
		// they belong in this build of the list, not only the next one.
		if reloaded, err := accountstore.Load(); err == nil {
			stored = reloaded
		}
	}

	discovered := 0
	for _, rec := range stored {
		if _, had := known[rec.SteamID64]; !had {
			discovered++
		}
	}
	if discovered > 0 {
		// An account nobody has fetched yet has no name and no avatar, and
		// nothing else is going to ask: the profile refresh runs on a page load
		// or a user action, and a folder appearing underneath the app is
		// neither. The next pass finds nothing new, so this cannot cycle.
		steamLog.Info("importing accounts the switcher had not seen", slog.Int("count", discovered))
		RequestProfileRefresh()
	}

	present := make(map[string]struct{}, len(users))
	for _, u := range users {
		if id := strings.TrimSpace(u.SteamID64); id != "" {
			present[id] = struct{}{}
		}
	}

	out := append([]LoginUser(nil), users...)
	for _, rec := range stored {
		if _, ok := present[rec.SteamID64]; ok {
			continue
		}
		out = append(out, loginUserFromRecord(rec))
	}
	return out
}

// recordsFromLoginUsers keeps only what identifies an account. MostRecent,
// AutoLogin and RememberPassword are live session state that belongs to Steam's
// file; storing them would let a stale copy fight the real one.
func recordsFromLoginUsers(users []LoginUser) []accountstore.Record {
	out := make([]accountstore.Record, 0, len(users))
	for _, u := range users {
		out = append(out, accountstore.Record{
			SteamID64:       strings.TrimSpace(u.SteamID64),
			AccountName:     u.AccountName,
			PersonaName:     u.PersonaName,
			Timestamp:       u.Timestamp,
			WantsOffline:    u.WantsOffline,
			SkipOfflineWarn: u.SkipOfflineWarn,
			Source:          accountstore.SourceVDF,
		})
	}
	return out
}

// recordsFromSteamGuardRegistry imports accounts that exist only in the Steam
// Guard registration index. Restoring a backup, or swapping the SteamGuard
// folder in by hand, puts accounts there without passing through any of the
// paths that seed the store, and an account the switcher cannot see is one the
// restore looks to have dropped.
//
// The index holds no name, so these arrive bare; the profile refresh fills in a
// community name and an avatar, and opening the Steam Guard picker contributes
// the login name from the unlocked vault.
func recordsFromSteamGuardRegistry() []accountstore.Record {
	entries, err := steamguardregistry.Load()
	if err != nil {
		steamLog.Warn("Steam Guard account state unavailable", slog.Any("err", err))
		return nil
	}
	out := make([]accountstore.Record, 0, len(entries))
	for _, entry := range entries {
		id := strings.TrimSpace(entry.SteamID64)
		if id == "" {
			continue
		}
		out = append(out, accountstore.Record{SteamID64: id, Source: accountstore.SourceSteamGuard})
	}
	return out
}

// loginUserFromRecord rebuilds a loginusers.vdf row from the store. The session
// markers are cleared so ActiveSessionSteamID64 still reads the live account out
// of Steam's own rows: an account Steam does not list cannot be the one running.
func loginUserFromRecord(rec accountstore.Record) LoginUser {
	return LoginUser{
		SteamID64:       rec.SteamID64,
		PersonaName:     rec.PersonaName,
		AccountName:     rec.AccountName,
		Timestamp:       rec.Timestamp,
		WantsOffline:    rec.WantsOffline,
		SkipOfflineWarn: rec.SkipOfflineWarn,
		MostRecent:      "0",
		AutoLogin:       "0",
	}
}

// knownAccountAsLoginUser materialises a stored account as a row ready to be
// appended to loginusers.vdf. It reports false when the switcher has never seen
// the account either.
func knownAccountAsLoginUser(steamID64 string) (LoginUser, bool) {
	rec, ok, err := accountstore.Get(steamID64)
	if err != nil {
		steamLog.Warn("Steam account store unavailable",
			slog.String("steamId", tailSteamID(steamID64)), slog.Any("err", err))
		return LoginUser{}, false
	}
	if !ok {
		return LoginUser{}, false
	}
	u := loginUserFromRecord(rec)
	if strings.TrimSpace(u.AccountName) != "" {
		return u, true
	}
	// Imported from the Steam Guard registration index, which stores no name.
	// Ask the vault, and keep what it says: the next switch then works whether
	// or not the vault happens to be open.
	name, found := resolveAccountName(rec.SteamID64)
	if !found {
		return u, true
	}
	u.AccountName = name
	if _, err := accountstore.Upsert(accountstore.Record{SteamID64: rec.SteamID64, AccountName: name}); err != nil {
		steamLog.Warn("could not record a recovered Steam login name",
			slog.String("steamId", tailSteamID(rec.SteamID64)), slog.Any("err", err))
	}
	return u, true
}

// ErrSwitchTargetHasNoLoginName is a switch to an account whose Steam login name
// nothing can supply. Steam signs in by login name, so there is nothing useful
// to write; saying so beats closing Steam and reopening it unchanged.
var ErrSwitchTargetHasNoLoginName = errors.New("steam login name unknown: unlock Steam Guard once so the account can be signed in")

// preflightSwitchTarget refuses a switch that could not work, BEFORE Steam is
// closed for it. The row is written after the kill, so without this check a
// nameless account tore down the running session and put nothing in its place.
func preflightSwitchTarget(steamRoot, selected string) error {
	if selected == "" {
		return nil
	}
	users, err := ParseLoginUsers(LoginUsersPath(steamRoot))
	if err != nil && !os.IsNotExist(err) {
		// Unreadable is the switch path's own problem to report later.
		return nil
	}
	if hasLoginUser(users, selected) {
		return nil
	}
	u, ok := knownAccountAsLoginUser(selected)
	if !ok {
		return fmt.Errorf("steam account %s is unknown to Steam and to the switcher", selected)
	}
	if strings.TrimSpace(u.AccountName) == "" {
		return ErrSwitchTargetHasNoLoginName
	}
	return nil
}
