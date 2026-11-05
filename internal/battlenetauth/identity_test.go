package battlenetauth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestEnrollmentReadsIdentityBeforeCreatingAuthenticator(t *testing.T) {
	c := NewClient()
	stage := 0
	c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"access_token":"fixture-bearer"}`
		switch stage {
		case 0:
			if r.URL.Path != "/oauth/sso" {
				t.Fatal("unexpected sign-in endpoint")
			}
		case 1:
			if r.Method != http.MethodGet || r.URL.String() != "https://oauth.battle.net/userinfo" || r.Header.Get("Authorization") != "Bearer fixture-bearer" || r.ContentLength != 0 {
				t.Fatal("unsafe identity request")
			}
			body = `{"battletag":"Example#1234","email":"fixture@example.test"}`
		case 3:
			if r.Method != http.MethodPost || r.URL.Host != "authenticator-rest-api.bnet-identity.blizzard.net" {
				t.Fatal("unexpected enrollment endpoint")
			}
			body = `{"serial":"EU123456789012","deviceSecret":"3132333435363738393031323334353637383930","restoreCode":"0123456789"}`
		default:
			t.Fatal("identity was not accepted before enrollment")
		}
		stage++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	a, err := c.EnrollWithRecovery(context.Background(), "EU-012345678901234567890123456789", func(raw []byte) error {
		if stage != 4 || strings.Contains(string(raw), "fixture-bearer") {
			t.Fatal("incorrect recovery response")
		}
		stage++
		return nil
	}, func(identity EnrollmentIdentity) error {
		if stage != 2 || identity.BattleTag != "Example#1234" || identity.Email != "fixture@example.test" {
			t.Fatal("missing preflight identity")
		}
		stage++
		return nil
	})
	if err != nil || stage != 5 || a.DisplayName() != "Example#1234" || a.AccountLink != "fixture@example.test" {
		t.Fatal("enrollment lost preflight identity", err)
	}
}

func TestEnrollmentIdentityFailureNeverCreatesAuthenticator(t *testing.T) {
	for _, body := range []string{`{"battletag":"Example#1234"}`, `{"sub":"123"}`, `{"battletag":"bad"}`, `{`} {
		for _, status := range []int{200, 403} {
			c := NewClient()
			calls := 0
			c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"fixture-bearer"}`))}, nil
				}
				if calls != 2 || r.URL.Path != "/userinfo" {
					t.Fatal("created authenticator after identity failed")
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			_, err := c.EnrollWithRecovery(context.Background(), "EU-012345678901234567890123456789", func([]byte) error { t.Fatal("preflight produced a recovery receipt"); return nil }, func(EnrollmentIdentity) error { return ErrAccountIdentity })
			if !errors.Is(err, ErrAccountIdentity) || calls != 2 {
				t.Fatal("identity failure was retried or ignored")
			}
		}
	}
}

func TestRecoveryRetainsEnrollmentIdentity(t *testing.T) {
	raw, err := EncodeRecovery([]byte(`{"serial":"EU123456789012","deviceSecret":"3132333435363738393031323334353637383930","restoreCode":"0123456789"}`), "battlenet-fixture", "", "", EnrollmentIdentity{BattleTag: "Example#1234", Email: "fixture@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := RecoverAccount(raw)
	if err != nil || a.DisplayName() != "Example#1234" || a.AccountLink != "fixture@example.test" || a.BrowserProfile != "battlenet-fixture" {
		t.Fatal("recovery lost account identity", err)
	}
}

func TestEnrollmentIdentityValidation(t *testing.T) {
	for _, tag := range []string{"Example#1234", "玩家#1234567"} {
		if (EnrollmentIdentity{BattleTag: tag}).Validate() != nil {
			t.Fatal("valid BattleTag rejected")
		}
	}
	for _, tag := range []string{"", "Example", "Example#12", "Example#123x", "Example\n#1234", "Example\u200b#1234"} {
		if (EnrollmentIdentity{BattleTag: tag}).Validate() == nil {
			t.Fatal("invalid BattleTag accepted")
		}
	}
}
