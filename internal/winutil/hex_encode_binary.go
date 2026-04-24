package winutil

import (
	"encoding/hex"
	"strings"
)

// HexEncodeBinary formats bytes as "(hex) aa bb cc" for JSON round-trip.
func HexEncodeBinary(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return "(hex) " + strings.TrimSpace(hex.EncodeToString(b))
}
