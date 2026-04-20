package steam

import (
	"fmt"

	"github.com/Jleagle/steam-go/steamid"
)

// SteamIDFormats exposes common string forms for a SteamID64 decimal string.
type SteamIDFormats struct {
	ID64   string
	ID3    string
	STEAMx string
	ID32   string
}

// FormatsFromID64 parses decimal SteamID64 and returns display formats.
func FormatsFromID64(id64 string) (SteamIDFormats, error) {
	sid, err := steamid.ParsePlayerID(id64)
	if err != nil {
		return SteamIDFormats{ID64: id64}, err
	}
	acc := uint32(sid.GetAccountID())
	u := sid.GetUniverseID()
	low := acc & 1
	high := acc >> 1
	return SteamIDFormats{
		ID64:   sid.String(),
		ID3:    fmt.Sprintf("[U:1:%d]", acc),
		STEAMx: fmt.Sprintf("STEAM_%d:%d:%d", u, low, high),
		ID32:   fmt.Sprintf("%d", acc),
	}, nil
}
