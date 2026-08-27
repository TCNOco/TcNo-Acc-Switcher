package sessionrefresh

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

// AccessTokenExpiry reads the expiry out of a Steam web access token. Steam issues
// JWTs, so the claim is readable without contacting Steam — enough to tell a caller
// that a stored session cannot work any more before spending a request to find out.
//
// It reports false for anything it cannot read: an unreadable token is not evidence
// of expiry, and the caller decides what to do with the absence. The token is never
// logged, retained, or returned in any form.
func AccessTokenExpiry(token string) (time.Time, bool) {
	token = strings.TrimSpace(token)
	if token == "" || len(token) > maxTokenBytes {
		return time.Time{}, false
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return time.Time{}, false
	}
	// Deliberately tolerant of unknown claims: Steam adds them, and rejecting a
	// token for carrying one would report a working session as unusable.
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp <= 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0).UTC(), true
}

// AccessTokenExpired reports whether a token's own expiry has already passed.
// skew absorbs clock drift between this machine and Steam, so a token about to
// lapse is treated as lapsed rather than spending a request that will be refused.
func AccessTokenExpired(token string, now time.Time, skew time.Duration) bool {
	expiry, ok := AccessTokenExpiry(token)
	if !ok {
		return false
	}
	return !expiry.After(now.Add(skew))
}

// RefreshTokenUsable reports whether a stored refresh token can still mint a new
// access token without the user signing in again.
//
// Steam issues both as JWTs, so this is the same local expiry read, but the two
// run on very different clocks: an access token lasts about a day against a
// refresh token's months. An account whose access token lapsed overnight is not
// an account that needs a password.
//
// An unreadable token counts as usable. Absence of a claim this build can parse
// is not evidence of expiry, and the cost of being wrong is one refused refresh
// that falls back to the sign-in already on offer.
func RefreshTokenUsable(token string, now time.Time, skew time.Duration) bool {
	if strings.TrimSpace(token) == "" {
		return false
	}
	if _, readable := AccessTokenExpiry(token); !readable {
		return true
	}
	return !AccessTokenExpired(token, now, skew)
}
