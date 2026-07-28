package sessionrefresh

import (
	"encoding/base64"
	"testing"
	"time"
)

func testToken(t *testing.T, payload string) string {
	t.Helper()
	segment := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return "header." + segment + ".signature"
}

func TestAccessTokenExpiryReadsTheClaim(t *testing.T) {
	t.Parallel()

	token := testToken(t, `{"exp":1750000000,"sub":"7656119","aud":["web"]}`)
	expiry, ok := AccessTokenExpiry(token)
	if !ok || !expiry.Equal(time.Unix(1750000000, 0).UTC()) {
		t.Fatalf("expiry = %v, ok = %v", expiry, ok)
	}
}

// An unreadable token is not evidence of expiry: reporting one as expired would
// send the user to a sign-in they may not need.
func TestAccessTokenExpiryReportsNothingItCannotRead(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name  string
		token string
	}{
		{name: "empty", token: ""},
		{name: "not a jwt", token: "opaque-token"},
		{name: "bad base64", token: "header.!!!.signature"},
		{name: "not json", token: "header." + base64.RawURLEncoding.EncodeToString([]byte("nope")) + ".sig"},
		{name: "no exp claim", token: "header." + base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"1"}`)) + ".sig"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if _, ok := AccessTokenExpiry(testCase.token); ok {
				t.Fatal("unreadable token reported an expiry")
			}
			if AccessTokenExpired(testCase.token, time.Now(), time.Minute) {
				t.Fatal("unreadable token reported as expired")
			}
		})
	}
}

func TestAccessTokenExpiredHonoursSkew(t *testing.T) {
	t.Parallel()

	expiry := time.Unix(1750000000, 0).UTC()
	token := testToken(t, `{"exp":1750000000}`)

	if AccessTokenExpired(token, expiry.Add(-time.Hour), time.Minute) {
		t.Fatal("token with an hour left reported as expired")
	}
	if !AccessTokenExpired(token, expiry.Add(-30*time.Second), time.Minute) {
		t.Fatal("token inside the skew window should count as expired")
	}
	if !AccessTokenExpired(token, expiry.Add(time.Second), 0) {
		t.Fatal("token past its expiry reported as usable")
	}
}
