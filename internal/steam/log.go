package steam

import (
	"log/slog"
	"strings"
)

// steamLog is the structured logger for this package (Steam account sync, VDF, HTTP).
var steamLog = slog.Default().With("component", "steam")

// tailSteamID logs only the last 6 digits to reduce PII in logs.
func tailSteamID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 6 {
		return "****"
	}
	return "…" + id[len(id)-6:]
}
