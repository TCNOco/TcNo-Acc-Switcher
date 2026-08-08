package steam

import (
	"log/slog"
	"strings"

	"TcNo-Acc-Switcher/internal/steam/accountstore"
)

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

	if _, err := accountstore.UpsertMany(recordsFromLoginUsers(users)); err != nil {
		steamLog.Warn("could not update the Steam account store", slog.Any("err", err))
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
