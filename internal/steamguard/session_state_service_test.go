package steamguard

import (
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/vaultrecord"
)

func TestLocalSessionStatusClassifiesWithoutAskingSteam(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	lapsed := accessTokenExpiringAt(now.Add(-time.Second))

	for _, tc := range []struct {
		name    string
		kind    vaultrecord.Kind
		token   string
		refresh string
		want    SessionStatus
	}{
		{"live session", vaultrecord.KindMaFile, accessTokenExpiringAt(now.Add(time.Hour)), "", SessionStatusValid},
		{"lapsed session with nothing to renew from", vaultrecord.KindMaFile, lapsed, "", SessionStatusNeedsLogin},
		// Within the skew the token is treated as already gone, so the row and the
		// screen it opens cannot land on opposite sides of the boundary.
		{"lapsing inside the skew", vaultrecord.KindLoginOnly, accessTokenExpiringAt(now.Add(accessTokenSkew - time.Second)), "", SessionStatusNeedsLogin},
		{"just outside the skew", vaultrecord.KindLoginOnly, accessTokenExpiringAt(now.Add(accessTokenSkew + time.Second)), "", SessionStatusValid},
		{"no session stored", vaultrecord.KindMaFile, "", "", SessionStatusNeedsLogin},
		// An unreadable token is not evidence either way.
		{"token this build cannot read", vaultrecord.KindMaFile, "not-a-jwt", "", SessionStatusUnknown},
		// A half-finished enrollment has no session to judge, and must not be
		// shown as needing a sign-in it cannot complete from the picker.
		{"pending enrollment", vaultrecord.KindEnrollmentPending, "", "", SessionStatusUnknown},

		// The two tokens run on different clocks. An access token ages out
		// overnight; the refresh token beside it lasts months and renews the
		// session with no user involvement, so these rows must not ask for one.
		{
			"lapsed access token renewable from a live refresh token",
			vaultrecord.KindMaFile, lapsed, accessTokenExpiringAt(now.Add(200 * 24 * time.Hour)), SessionStatusValid,
		},
		{
			"no access token but a live refresh token",
			vaultrecord.KindLoginOnly, "", accessTokenExpiringAt(now.Add(30 * 24 * time.Hour)), SessionStatusValid,
		},
		{
			"both tokens lapsed",
			vaultrecord.KindMaFile, lapsed, accessTokenExpiringAt(now.Add(-time.Hour)), SessionStatusNeedsLogin,
		},
		{
			"refresh token this build cannot read is not evidence of expiry",
			vaultrecord.KindMaFile, lapsed, "not-a-jwt", SessionStatusValid,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := localSessionStatus(tc.kind, tc.token, tc.refresh, now); got != tc.want {
				t.Fatalf("localSessionStatus = %q, want %q", got, tc.want)
			}
		})
	}
}
