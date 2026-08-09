package steam

import (
	"log/slog"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// OwnedGamesUpdatedEvent announces that one account's stored library changed.
//
// Only the count travels with it. The games view renders a join of every
// account's library against the app name map, so a per-account delta would not
// be enough to patch it in place - it reloads, and a few thousand app ids on the
// event bus would only make that slower.
const OwnedGamesUpdatedEvent = "steam-owned-games-updated"

type OwnedGamesPatch struct {
	SteamID64 string `json:"steamId64"`
	AppCount  int    `json:"appCount"`
}

// EmitOwnedGamesPatch publishes a library change to an open games view.
//
// Package-level because the sweep runs in the Steam Guard service and has no
// *SteamService to hang it off.
func EmitOwnedGamesPatch(patch OwnedGamesPatch) {
	app := application.Get()
	if app == nil {
		steamLog.Warn("emit steam-owned-games-updated skipped: application not ready",
			slog.String("steamId", tailSteamID(patch.SteamID64)))
		return
	}
	if strings.TrimSpace(patch.SteamID64) == "" {
		return
	}
	app.Event.Emit(OwnedGamesUpdatedEvent, patch)
}
