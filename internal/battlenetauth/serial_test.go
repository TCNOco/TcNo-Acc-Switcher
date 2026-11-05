package battlenetauth

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCompactSerialEnrollmentAndImports(t *testing.T) {
	const response = `{"serial":"EU123456789012","restoreCode":"0123456789","deviceSecret":"3132333435363738393031323334353637383930"}`
	c := NewClient()
	calls := 0
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
	a, err := c.Enroll(context.Background(), "EU-012345678901234567890123456789")
	if err != nil || a.Serial != "EU-1234-5678-9012" || a.Validate() != nil {
		t.Fatal("compact API serial was not accepted")
	}
	for _, raw := range []string{response, `{"version":1,"entries":{"EU123456789012":` + response + `}}`} {
		accounts, err := Import([]byte(raw), "")
		if err != nil || len(accounts) != 1 || accounts[0].Serial != a.Serial {
			t.Fatal("compact backup serial was not accepted")
		}
	}
	if calls != 3 {
		t.Fatal("enrollment retried")
	}
}

func TestSerialNormalizationIsStrict(t *testing.T) {
	for _, raw := range []string{"EU123456789012", "EU-1234-5678-9012", " eu123456789012 ", "EU 1234 5678 9012", " eu\u00a01234\t5678\n9012 ", "EU - 1234 - 5678 - 9012"} {
		if got, err := NormalizeSerial(raw); err != nil || got != "EU-1234-5678-9012" {
			t.Fatal("valid serial rejected")
		}
	}
	for _, raw := range []string{"XX123456789012", "EU12345678901", "EU1234567890123", "EU--1234-5678-9012", "EU/1234/5678/9012", "EU12345678901a"} {
		if _, err := NormalizeSerial(raw); err == nil {
			t.Fatal("malformed serial accepted")
		}
	}
}

func TestRestoreNormalizesSpacedSerial(t *testing.T) {
	c := NewClient()
	calls := 0
	c.http.Transport = transportFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		body, err := io.ReadAll(req.Body)
		if err != nil || !strings.Contains(string(body), `"serial":"EU-1234-5678-9012"`) {
			t.Fatal("restore request did not use the canonical serial")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"deviceSecret":"3132333435363738393031323334353637383930"}`)), Header: http.Header{}}, nil
	})
	a, err := c.Restore(context.Background(), "test@example.invalid", "EU 1234 5678 9012", "0123456789")
	if err != nil || a.Serial != "EU-1234-5678-9012" || a.AccountName != "test@example.invalid" || calls != 1 {
		t.Fatal("restore rejected a spaced serial or retried the request")
	}
}
