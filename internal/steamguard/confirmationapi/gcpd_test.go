package confirmationapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func htmlResponse(status int, body string) *http.Response {
	return jsonResponse(status, body)
}

func TestFetchCS2GCPDBuildsTheExactRequest(t *testing.T) {
	transport := &queuedTransport{responses: []*http.Response{htmlResponse(http.StatusOK, "<html></html>")}}
	client := testClient(transport, func() bool { return false })

	body, err := client.FetchCS2GCPD(context.Background(), testCredentials())
	if err != nil {
		t.Fatalf("FetchCS2GCPD: %v", err)
	}
	if string(body) != "<html></html>" {
		t.Fatalf("body = %q", body)
	}

	request := transport.requests[0]
	wantURL := "https://steamcommunity.com/profiles/76561198000000000/gcpd/730?tab=matchmaking"
	if request.Method != http.MethodGet || request.URL.String() != wantURL {
		t.Fatalf("request = %s %s", request.Method, request.URL)
	}
	cookie := request.Header.Get("Cookie")
	for _, want := range []string{
		"steamLoginSecure=76561198000000000%7C%7CeyJhbGciOiJIUzI1NiJ9.payload.signature",
		"sessionid=0123456789ABCDEF0123456789ABCDEF",
		// The parser identifies the cooldown table by its English header, so a
		// localised render would read as "no cooldown" and clear a real one.
		"Steam_Language=english",
	} {
		if !strings.Contains(cookie, want) {
			t.Fatalf("Cookie = %q, missing %q", cookie, want)
		}
	}
	if request.Header.Get("User-Agent") != MobileUserAgent {
		t.Fatalf("User-Agent = %q", request.Header.Get("User-Agent"))
	}
	if strings.Contains(request.URL.String(), testCredentials().AccessToken) {
		t.Fatal("access token leaked into URL")
	}
	if request.Header.Get("Authorization") != "" {
		t.Fatal("unexpected Authorization header")
	}
}

func TestFetchCS2GCPDAcceptsCredentialsWithoutAuthenticatorSecrets(t *testing.T) {
	// The whole point of the narrower validator: a login-only record has no
	// device id and no identity secret, and an unsigned GET needs neither.
	credentials := testCredentials()
	credentials.DeviceID = ""
	credentials.IdentitySecret = ""

	transport := &queuedTransport{responses: []*http.Response{htmlResponse(http.StatusOK, "<html></html>")}}
	client := testClient(transport, func() bool { return false })
	if _, err := client.FetchCS2GCPD(context.Background(), credentials); err != nil {
		t.Fatalf("FetchCS2GCPD: %v", err)
	}
}

func TestFetchCS2GCPDRejectsUnusableCredentials(t *testing.T) {
	cases := map[string]func(*Credentials){
		"no steam id":       func(c *Credentials) { c.SteamID = "" },
		"non-numeric id":    func(c *Credentials) { c.SteamID = "not-a-number" },
		"short token":       func(c *Credentials) { c.AccessToken = "short" },
		"unsafe token":      func(c *Credentials) { c.AccessToken = "has a space and stuff" },
		"no session id":     func(c *Credentials) { c.SessionID = "" },
		"non-hex sessionid": func(c *Credentials) { c.SessionID = "zzzzzzzzzzzzzzzzzzzzzzzz" },
	}
	for name, mutate := range cases {
		credentials := testCredentials()
		mutate(&credentials)
		transport := &queuedTransport{}
		client := testClient(transport, func() bool { return false })
		if _, err := client.FetchCS2GCPD(context.Background(), credentials); err == nil {
			t.Fatalf("%s: FetchCS2GCPD succeeded, want error", name)
		}
		if len(transport.requests) != 0 {
			t.Fatalf("%s: sent %d requests, want none", name, len(transport.requests))
		}
	}
}

// Every account with a custom Steam URL is answered with a 302 to /id/<vanity>/,
// cookie or no cookie. Denying that hop meant this page was never read for those
// accounts, and the sweep counted them as checked - so cooldowns, ranks and
// Prime silently never arrived for them.
func TestFetchCS2GCPDFollowsTheVanityRedirect(t *testing.T) {
	transport := &queuedTransport{responses: []*http.Response{
		jsonResponse(http.StatusFound, "", http.Header{
			"Location": {"https://steamcommunity.com/id/vanity/gcpd/730?tab=matchmaking"},
		}),
		htmlResponse(http.StatusOK, "<html>Personal Game Data</html>"),
	}}
	client := testClient(transport, func() bool { return false })

	body, err := client.FetchCS2GCPD(context.Background(), testCredentials())
	if err != nil {
		t.Fatalf("FetchCS2GCPD: %v", err)
	}
	if string(body) != "<html>Personal Game Data</html>" {
		t.Fatalf("body = %q", body)
	}
	if len(transport.requests) != 2 {
		t.Fatalf("made %d requests, want 2", len(transport.requests))
	}
	followed := transport.requests[1]
	if !strings.Contains(followed.Header.Get("Cookie"), "steamLoginSecure=") {
		t.Fatalf("session cookie did not survive the hop: %q", followed.Header.Get("Cookie"))
	}
	// The parser reads the cooldown table by its English header, so the language
	// pin has to survive the hop too or a localised render reads as "no cooldown".
	if !strings.Contains(followed.Header.Get("Cookie"), "Steam_Language=english") {
		t.Fatalf("language pin did not survive the hop: %q", followed.Header.Get("Cookie"))
	}
}

func TestFetchCS2GCPDClassifiesFailures(t *testing.T) {
	cases := map[string]struct {
		response *http.Response
		want     FailureKind
	}{
		// Redirects are followed now, but only ones that say where to. A 3xx with
		// no Location is not a hop, so it still surfaces as reauth rather than as
		// a body we might misparse.
		"redirect without a target": {htmlResponse(http.StatusFound, ""), FailureReauth},
		"unauthentic":               {htmlResponse(http.StatusUnauthorized, ""), FailureReauth},
		"rate limit":                {htmlResponse(http.StatusTooManyRequests, ""), FailureRateLimit},
	}
	for name, tc := range cases {
		transport := &queuedTransport{responses: []*http.Response{tc.response}}
		client := testClient(transport, func() bool { return false })
		_, err := client.FetchCS2GCPD(context.Background(), testCredentials())
		var apiErr *Error
		if !errors.As(err, &apiErr) {
			t.Fatalf("%s: err = %v, want *Error", name, err)
		}
		if apiErr.Kind != tc.want {
			t.Fatalf("%s: Kind = %v, want %v", name, apiErr.Kind, tc.want)
		}
	}
}

func TestFetchCS2GCPDRefusesWhenOffline(t *testing.T) {
	transport := &queuedTransport{}
	client := testClient(transport, func() bool { return true })
	_, err := client.FetchCS2GCPD(context.Background(), testCredentials())
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Kind != FailureOffline {
		t.Fatalf("err = %v, want FailureOffline", err)
	}
	if len(transport.requests) != 0 {
		t.Fatalf("sent %d requests while offline", len(transport.requests))
	}
}
