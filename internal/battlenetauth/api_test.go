package battlenetauth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSessionRedirectValidation(t *testing.T) {
	valid := "http://localhost/?ST=US-012345678901234567890123456789"
	if _, err := SessionTokenFromRedirect(valid); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"https://evil.test/?ST=US-012345678901234567890123456789", "http://localhost.evil.test/?ST=US-012345678901234567890123456789", "http://localhost:1234/?ST=US-012345678901234567890123456789", valid + "&ST=other", valid + "#fragment", "http://user@localhost/?ST=US-012345678901234567890123456789"} {
		if _, err := SessionTokenFromRedirect(raw); err == nil {
			t.Fatal("unsafe redirect accepted")
		}
	}
}

func TestBattleNetSessionRedirectValidation(t *testing.T) {
	const token = "EU-012345678901234567890123456789-1111111111"
	const callback = "https://account.battle.net/login/en/localhost"
	for _, raw := range []string{callback + "?ST=" + token + "&flowTrackingId=", callback + "/?ST=" + token} {
		if got, err := SessionTokenFromRedirect(raw); err != nil || got != token {
			t.Fatal("Battle.net session callback rejected")
		}
	}
	for _, raw := range []string{
		"http://account.battle.net/login/en/localhost?ST=" + token,
		"https://account.battle.net:8443/login/en/localhost?ST=" + token,
		"https://account.battle.net.evil.test/login/en/localhost?ST=" + token,
		"https://user@account.battle.net/login/en/localhost?ST=" + token,
		"https://oauth.battle.net/login/en/localhost?ST=" + token,
		"https://account.battle.net/security?ST=" + token,
		"https://account.battle.net/login/en/localhost.evil?ST=" + token,
		"https://account.battle.net/login/en/%6cocalhost?ST=" + token,
		callback + "?ST=" + token + "&ST=" + token,
		callback + "?ST=" + token + "&broken=%zz",
		callback + "?ST=" + token + "#fragment",
		callback + "?ST=invalid",
		callback,
	} {
		if got, err := SessionTokenFromRedirect(raw); err == nil || got != "" {
			t.Fatal("unsafe or malformed Battle.net callback accepted")
		}
	}
}

func TestEnrollmentDoesNotRetryOrExposeResponse(t *testing.T) {
	calls := 0
	c := NewClient()
	c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		body := `{"access_token":"fake-bearer"}`
		status := 200
		if calls == 3 {
			status = 500
			body = `{"deviceSecret":"do-not-expose-this"}`
		}
		if r.URL.Scheme != "https" || (r.Method != "POST" && r.URL.Path != "/userinfo") {
			t.Fatal("unsafe API request")
		}
		if r.URL.Path == "/userinfo" {
			body = `{"battletag":"Example#1234"}`
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})
	_, err := c.Enroll(context.Background(), "US-012345678901234567890123456789")
	if err == nil || calls != 3 || strings.Contains(err.Error(), "do-not-expose-this") {
		t.Fatal("enrollment retried or exposed response material")
	}
}

func TestEnrollmentReturnsPendingRecord(t *testing.T) {
	c := NewClient()
	calls := 0
	c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		body := `{"access_token":"fake-bearer"}`
		if calls == 3 {
			body = `{"serial":"US-1234-5678-9012","deviceSecret":"3132333435363738393031323334353637383930","restoreCode":"0123456789"}`
		}
		if r.URL.Path == "/userinfo" {
			body = `{"battletag":"Example#1234"}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})
	a, err := c.Enroll(context.Background(), "US-012345678901234567890123456789")
	if err != nil || !a.Pending || calls != 3 {
		t.Fatal("enrollment did not produce a valid pending record", err)
	}
}

func TestAccountSetupFlagPreservesReturnedCredentials(t *testing.T) {
	for _, restore := range []bool{false, true} {
		t.Run(map[bool]string{false: "enroll", true: "restore"}[restore], func(t *testing.T) {
			calls := 0
			c := NewClient()
			c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				body := `{"serial":"EU-1234-5678-9012","restoreCode":"0123456789","deviceSecret":"3132333435363738393031323334353637383930","requireHealup":true}`
				if !restore && calls == 1 {
					body = `{"access_token":"fake-bearer"}`
				}
				if r.URL.Path == "/userinfo" {
					body = `{"battletag":"Example#1234"}`
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
			})
			var a Account
			var err error
			if restore {
				a, err = c.Restore(context.Background(), "fixture@example.test", "EU-1234-5678-9012", "0123456789")
			} else {
				a, err = c.Enroll(context.Background(), "EU-012345678901234567890123456789")
			}
			if err != nil || !a.RequiresAccountSetup || !a.Pending || a.Validate() != nil {
				t.Fatal("credentials were discarded because account setup is required")
			}
			raw, err := json.Marshal(a)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := ParseAccount(raw)
			if err != nil || !restored.RequiresAccountSetup || restored.DeviceSecret != a.DeviceSecret {
				t.Fatal("saved account lost credentials or setup notice")
			}
			if (restore && calls != 1) || (!restore && calls != 3) {
				t.Fatal("request retried")
			}
		})
	}
}

func TestAccountSetupWithoutCredentialsStopsWithoutRetry(t *testing.T) {
	for _, stage := range []string{"sign-in", "enroll", "restore"} {
		t.Run(stage, func(t *testing.T) {
			calls := 0
			c := NewClient()
			c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				body := `{"requireHealup":true,"private":"must-not-appear"}`
				if stage == "enroll" && calls == 1 {
					body = `{"access_token":"fake-bearer"}`
				}
				if r.URL.Path == "/userinfo" {
					body = `{"battletag":"Example#1234"}`
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
			})
			var err error
			if stage == "restore" {
				_, err = c.Restore(context.Background(), "fixture@example.test", "EU-1234-5678-9012", "0123456789")
			} else {
				_, err = c.Enroll(context.Background(), "EU-012345678901234567890123456789")
			}
			if !errors.Is(err, ErrAccountVerification) || strings.Contains(err.Error(), "must-not-appear") {
				t.Fatal("incorrect or sensitive setup error")
			}
			want := 1
			if stage == "enroll" {
				want = 3
			}
			if calls != want {
				t.Fatal("verification requirement caused extra requests")
			}
		})
	}
}
