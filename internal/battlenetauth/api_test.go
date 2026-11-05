package battlenetauth

import (
	"context"
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

func TestEnrollmentDoesNotRetryOrExposeResponse(t *testing.T) {
	calls := 0
	c := NewClient()
	c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		body := `{"access_token":"fake-bearer"}`
		status := 200
		if calls == 2 {
			status = 500
			body = `{"deviceSecret":"do-not-expose-this"}`
		}
		if r.URL.Scheme != "https" || r.Method != "POST" {
			t.Fatal("unsafe API request")
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})
	_, err := c.Enroll(context.Background(), "US-012345678901234567890123456789")
	if err == nil || calls != 2 || strings.Contains(err.Error(), "do-not-expose-this") {
		t.Fatal("enrollment retried or exposed response material")
	}
}

func TestEnrollmentReturnsPendingRecord(t *testing.T) {
	c := NewClient()
	calls := 0
	c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		body := `{"access_token":"fake-bearer"}`
		if calls == 2 {
			body = `{"serial":"US-1234-5678-9012","deviceSecret":"3132333435363738393031323334353637383930","restoreCode":"0123456789"}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})
	a, err := c.Enroll(context.Background(), "US-012345678901234567890123456789")
	if err != nil || !a.Pending || calls != 2 {
		t.Fatal("enrollment did not produce a valid pending record", err)
	}
}
