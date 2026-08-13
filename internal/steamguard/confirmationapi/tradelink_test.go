package confirmationapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"TcNo-Acc-Switcher/internal/steamguard/protocol"
)

func TestFetchTradeOfferPrivacyPageBuildsTheExactRequest(t *testing.T) {
	transport := &queuedTransport{responses: []*http.Response{htmlResponse(http.StatusOK, "<html></html>")}}
	client := testClient(transport, func() bool { return false })

	body, err := client.FetchTradeOfferPrivacyPage(context.Background(), testCredentials())
	if err != nil {
		t.Fatalf("FetchTradeOfferPrivacyPage: %v", err)
	}
	if string(body) != "<html></html>" {
		t.Fatalf("body = %q", body)
	}

	request := transport.requests[0]
	wantURL := "https://steamcommunity.com/profiles/76561198000000000/tradeoffers/privacy"
	if request.URL.String() != wantURL {
		t.Fatalf("url = %s, want %s", request.URL, wantURL)
	}
	// A POST to this same page rotates the trade token and breaks every link the
	// user has already handed out. Reading it must never be able to do that.
	if request.Method != http.MethodGet {
		t.Fatalf("method = %s, want GET", request.Method)
	}
	cookie := request.Header.Get("Cookie")
	for _, want := range []string{
		"steamLoginSecure=76561198000000000%7C%7CeyJhbGciOiJIUzI1NiJ9.payload.signature",
		"sessionid=0123456789ABCDEF0123456789ABCDEF",
		"Steam_Language=english",
	} {
		if !strings.Contains(cookie, want) {
			t.Fatalf("Cookie = %q, missing %q", cookie, want)
		}
	}
	// Under the mobile shell Steam bounces this page between forms until the
	// redirect budget runs out, and that reads as a refused session on an account
	// that is signed in perfectly well.
	if strings.Contains(cookie, "mobileClient") {
		t.Fatalf("mobile client marker asks for the mobile shell: %q", cookie)
	}
	if request.Header.Get("User-Agent") != protocol.UserAgent {
		t.Fatalf("User-Agent = %q, want the desktop agent", request.Header.Get("User-Agent"))
	}
	if strings.Contains(request.URL.String(), testCredentials().AccessToken) {
		t.Fatal("access token leaked into URL")
	}
}

func TestFetchTradeOfferPrivacyPageAcceptsCredentialsWithoutAuthenticatorSecrets(t *testing.T) {
	// A login-only record has no device id and no identity secret. Both are
	// mobileconf signing inputs, and this request is unsigned, so it must work
	// for those accounts too.
	credentials := testCredentials()
	credentials.DeviceID = ""
	credentials.IdentitySecret = ""

	transport := &queuedTransport{responses: []*http.Response{htmlResponse(http.StatusOK, "<html></html>")}}
	client := testClient(transport, func() bool { return false })
	if _, err := client.FetchTradeOfferPrivacyPage(context.Background(), credentials); err != nil {
		t.Fatalf("FetchTradeOfferPrivacyPage: %v", err)
	}
}

func TestFetchTradeOfferPrivacyPageRejectsUnusableCredentials(t *testing.T) {
	cases := map[string]func(*Credentials){
		"no steam id":       func(c *Credentials) { c.SteamID = "" },
		"non-numeric id":    func(c *Credentials) { c.SteamID = "not-a-number" },
		"path traversal":    func(c *Credentials) { c.SteamID = "../../settings" },
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
		if _, err := client.FetchTradeOfferPrivacyPage(context.Background(), credentials); err == nil {
			t.Fatalf("%s: succeeded, want error", name)
		}
		if len(transport.requests) != 0 {
			t.Fatalf("%s: sent %d requests, want none", name, len(transport.requests))
		}
	}
}

// Any account with a custom URL is answered with a 302 to /id/<vanity>/..., so
// this is the ordinary path for a large share of accounts, not an edge case.
// The hop has to carry the cookies or it lands on the login page and a live
// session is reported as "sign in again".
func TestFetchTradeOfferPrivacyPageFollowsTheVanityRedirect(t *testing.T) {
	transport := &queuedTransport{responses: []*http.Response{
		jsonResponse(http.StatusFound, "", http.Header{
			"Location": {"https://steamcommunity.com/id/vanity/tradeoffers/privacy"},
		}),
		htmlResponse(http.StatusOK, "<html>the page</html>"),
	}}
	client := testClient(transport, func() bool { return false })

	body, err := client.FetchTradeOfferPrivacyPage(context.Background(), testCredentials())
	if err != nil {
		t.Fatalf("FetchTradeOfferPrivacyPage: %v", err)
	}
	if string(body) != "<html>the page</html>" {
		t.Fatalf("body = %q", body)
	}
	if len(transport.requests) != 2 {
		t.Fatalf("made %d requests, want 2", len(transport.requests))
	}
	followed := transport.requests[1]
	if followed.URL.String() != "https://steamcommunity.com/id/vanity/tradeoffers/privacy" {
		t.Fatalf("followed %s", followed.URL)
	}
	if !strings.Contains(followed.Header.Get("Cookie"), "steamLoginSecure=") {
		t.Fatalf("session cookie did not survive the hop: %q", followed.Header.Get("Cookie"))
	}
	// Still a read, on the hop as much as on the first request.
	if followed.Method != http.MethodGet {
		t.Fatalf("followed with %s, want GET", followed.Method)
	}
}

// Steam redirects a rejected session to its own login page, on the same host.
// Following it is right: the body that comes back is what tells the parser the
// session is dead, rather than a bare 302 that could mean either thing.
func TestFetchTradeOfferPrivacyPageFollowsALoginRedirectToItsBody(t *testing.T) {
	transport := &queuedTransport{responses: []*http.Response{
		jsonResponse(http.StatusFound, "", http.Header{
			"Location": {"https://steamcommunity.com/login/home/?goto=tradeoffers"},
		}),
		htmlResponse(http.StatusOK, "<html><title>Sign In</title></html>"),
	}}
	client := testClient(transport, func() bool { return false })

	body, err := client.FetchTradeOfferPrivacyPage(context.Background(), testCredentials())
	if err != nil {
		t.Fatalf("FetchTradeOfferPrivacyPage: %v", err)
	}
	if !strings.Contains(string(body), "Sign In") {
		t.Fatalf("body = %q", body)
	}
}

func TestFetchTradeOfferPrivacyPageClassifiesFailures(t *testing.T) {
	cases := map[string]struct {
		response *http.Response
		want     FailureKind
	}{
		"unauthentic": {htmlResponse(http.StatusUnauthorized, ""), FailureReauth},
		"rate limit":  {htmlResponse(http.StatusTooManyRequests, ""), FailureRateLimit},
	}
	for name, tc := range cases {
		transport := &queuedTransport{responses: []*http.Response{tc.response}}
		client := testClient(transport, func() bool { return false })
		_, err := client.FetchTradeOfferPrivacyPage(context.Background(), testCredentials())
		var apiErr *Error
		if !errors.As(err, &apiErr) {
			t.Fatalf("%s: err = %v, want *Error", name, err)
		}
		if apiErr.Kind != tc.want {
			t.Fatalf("%s: Kind = %v, want %v", name, apiErr.Kind, tc.want)
		}
	}
}

func TestFetchTradeOfferPrivacyPageRefusesWhenOffline(t *testing.T) {
	transport := &queuedTransport{}
	client := testClient(transport, func() bool { return true })
	_, err := client.FetchTradeOfferPrivacyPage(context.Background(), testCredentials())
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Kind != FailureOffline {
		t.Fatalf("err = %v, want FailureOffline", err)
	}
	if len(transport.requests) != 0 {
		t.Fatalf("sent %d requests while offline", len(transport.requests))
	}
}
