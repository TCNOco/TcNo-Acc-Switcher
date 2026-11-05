package battlenetauth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestEnrollmentResponseIsPreservedBeforeValidation(t *testing.T) {
	for _, response := range []string{
		`{"serial":"EU123456789012","deviceSecret":"unsupported-shape","restoreCode":"0123456789"}`,
		`{"serial":"EU123456789012","deviceSecret":"unsupported-shape"`,
	} {
		c := NewClient()
		calls, saves := 0, 0
		var preserved []byte
		c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			body := response
			if calls == 1 {
				body = `{"access_token":"fixture-bearer"}`
			}
			if r.URL.Path == "/userinfo" {
				body = `{"battletag":"Example#1234"}`
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
		})
		_, err := c.EnrollWithRecovery(context.Background(), "EU-012345678901234567890123456789", func(raw []byte) error {
			saves++
			preserved = append([]byte(nil), raw...)
			return nil
		})
		if !errors.Is(err, ErrInvalid) || calls != 3 || saves != 1 || !bytes.Equal(preserved, []byte(response)) {
			t.Fatal("invalid response was lost or enrollment was retried")
		}
	}
}

func TestRecoveryReconstructsCompactSerialAndRestoreInputs(t *testing.T) {
	for _, tc := range []struct{ response, serial, code string }{
		{`{"serial":"EU123456789012","deviceSecret":"3132333435363738393031323334353637383930","restoreCode":"0123456789"}`, "", ""},
		{`{"deviceSecret":"3132333435363738393031323334353637383930"}`, "EU123456789012", "0123456789"},
	} {
		raw, err := EncodeRecovery([]byte(tc.response), "battlenet-fixture", tc.serial, tc.code)
		if err != nil {
			t.Fatal(err)
		}
		a, err := RecoverAccount(raw)
		if err != nil || a.Serial != "EU-1234-5678-9012" || a.RestoreCode != "0123456789" || a.BrowserProfile != "battlenet-fixture" || !a.Pending {
			t.Fatal("saved response could not be reconstructed")
		}
	}
}

func TestSetupFlagDoesNotDiscardMalformedCredentials(t *testing.T) {
	raw, _ := EncodeRecovery([]byte(`{"requireHealup":true,"serial":"unsupported","deviceSecret":"private-fixture"}`), "battlenet-fixture", "", "")
	_, err := RecoverAccount(raw)
	if !errors.Is(err, ErrInvalid) || errors.Is(err, ErrAccountVerification) {
		t.Fatal("credential-bearing response was treated as a setup-only notice")
	}
}
