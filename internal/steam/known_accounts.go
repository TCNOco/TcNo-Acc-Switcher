package steam

import (
	"log/slog"
	"strings"

	"TcNo-Acc-Switcher/internal/steam/accountstore"
	steamguardregistry "TcNo-Acc-Switcher/internal/steamguard/registry"
)

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

	incoming := recordsFromLoginUsers(users)
	incoming = append(incoming, recordsFromSteamGuardRegistry()...)
	if changed, err := accountstore.UpsertMany(incoming); err != nil {
		steamLog.Warn("could not update the Steam account store", slog.Any("err", err))
	} else if changed {
		// The upsert can add accounts the load above could not see - a restored
		// Steam Guard folder is exactly that - and they belong in this build of
		// the list, not only the next one.
		if reloaded, err := accountstore.Load(); err == nil {
			stored = reloaded
		}
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
	return loginUserFromRecord(rec), true
}
