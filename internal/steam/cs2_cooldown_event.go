package steam

import (
	"log/slog"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"TcNo-Acc-Switcher/internal/basic"
)

// CS2CooldownUpdatedEvent carries a single account's CS2 cooldown change.
//
// This is deliberately not an AccountPatch. AccountPatch's Vac, Ltd, ImageURL
// and AvatarPending fields have no omitempty, so a cooldown-only patch would
// serialise them as false/"" and the frontend merge would apply them - blanking
// avatars and ban flags every time a cooldown changed.
const CS2CooldownUpdatedEvent = "steam-cs2-cooldown-updated"

type CS2CooldownPatch struct {
	SteamID64 string `json:"steamId64"`
	// CS2CooldownExpiresAt is RFC3339 UTC, or empty for no cooldown. It is an
	// absolute instant rather than a duration so the UI can count it down
	// without another round trip.
	CS2CooldownExpiresAt string `json:"cs2CooldownExpiresAt"`
	CS2CooldownPermanent bool   `json:"cs2CooldownPermanent"`
	// The account's tags after the managed CS2 Cooldown tag was applied or
	// removed. Sent whole rather than as a delta: the list holds the resolved
	// list per account, and a cooldown starting or ending is exactly when it goes
	// stale.
	Tags []basic.AccountTagDTO `json:"tags"`
}

// EmitCS2CooldownPatch publishes a cooldown change to an open account list.
//
// Package-level because the sweep runs in the Steam Guard service and has no
// *SteamService to hang it off.
func EmitCS2CooldownPatch(patch CS2CooldownPatch) {
	app := application.Get()
	if app == nil {
		steamLog().Warn("emit steam-cs2-cooldown-updated skipped: application not ready",
			slog.String("steamId", tailSteamID(patch.SteamID64)))
		return
	}
	if strings.TrimSpace(patch.SteamID64) == "" {
		return
	}
	app.Event.Emit(CS2CooldownUpdatedEvent, patch)
}
