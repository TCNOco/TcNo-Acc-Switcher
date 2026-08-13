package steamguard

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"TcNo-Acc-Switcher/internal/steam"
	"TcNo-Acc-Switcher/internal/steamguard/confirmationapi"
)

// fakeTradeLinkClient answers the one request a trade-link read makes and
// refuses every other, so a change that starts issuing extra requests fails
// loudly instead of quietly costing the user a round trip.
type fakeTradeLinkClient struct {
	body        []byte
	err         error
	credentials confirmationapi.Credentials
	calls       int
}

func (f *fakeTradeLinkClient) FetchTradeOfferPrivacyPage(_ context.Context, c confirmationapi.Credentials) ([]byte, error) {
	f.calls++
	f.credentials = c
	return f.body, f.err
}

func (f *fakeTradeLinkClient) FetchCS2GCPD(context.Context, confirmationapi.Credentials) ([]byte, error) {
	panic("a trade link read must not fetch the GCPD page")
}

func (f *fakeTradeLinkClient) FetchCS2StorePage(context.Context, confirmationapi.Credentials) ([]byte, error) {
	panic("a trade link read must not fetch the store page")
}

func (f *fakeTradeLinkClient) FetchOwnedApps(context.Context, confirmationapi.Credentials) ([]uint32, error) {
	panic("a trade link read must not fetch owned apps")
}

func (f *fakeTradeLinkClient) List(context.Context, confirmationapi.Credentials) ([]confirmationapi.Confirmation, error) {
	panic("a trade link read must not list confirmations")
}

func (f *fakeTradeLinkClient) FetchDetails(context.Context, confirmationapi.Credentials, confirmationapi.Confirmation) (confirmationapi.Details, error) {
	panic("a trade link read must not fetch confirmation details")
}

func (f *fakeTradeLinkClient) FetchItemClass(context.Context, confirmationapi.Credentials, string, string, string) (confirmationapi.ItemClass, error) {
	panic("a trade link read must not fetch item classes")
}

func (f *fakeTradeLinkClient) Decide(context.Context, confirmationapi.Credentials, confirmationapi.Confirmation, confirmationapi.Decision) error {
	panic("a trade link read must not decide confirmations")
}

func (f *fakeTradeLinkClient) DecideBatch(context.Context, confirmationapi.Credentials, []confirmationapi.Confirmation, confirmationapi.Decision) error {
	panic("a trade link read must not decide confirmations")
}

func (f *fakeTradeLinkClient) CloseIdleConnections() {}

// tradeLinkPage renders the privacy page for one account, escaped the way Steam
// serves it.
func tradeLinkPage(t *testing.T, steamID uint64, token string) []byte {
	t.Helper()
	formats, err := steam.FormatsFromID64(strconv.FormatUint(steamID, 10))
	if err != nil {
		t.Fatal(err)
	}
	return []byte(`<html><body><input id="trade_offer_access_url" value="` +
		`https://steamcommunity.com/tradeoffer/new/?partner=` + formats.ID32 +
		`&amp;token=` + token + `"></body></html>`)
}

// A full authenticator is the ordinary case: the link comes back canonicalised,
// and the request that produced it carried a cookie session rather than the
// mobileconf signing inputs this page does not need.
func TestGetSteamTradeLinkReadsTheLink(t *testing.T) {
	service, accountID, grant := newAuthServiceFixture(t)
	fake := &fakeTradeLinkClient{body: tradeLinkPage(t, authServiceSteamID, "aB3-_xYz")}
	service.confirmationClient = fake

	result, err := service.GetSteamTradeLink(accountID, grant.Capability)
	if err != nil {
		t.Fatalf("GetSteamTradeLink: %v", err)
	}
	if result.State != "ok" {
		t.Fatalf("state = %q, want ok", result.State)
	}
	formats, err := steam.FormatsFromID64(accountID)
	if err != nil {
		t.Fatal(err)
	}
	want := "https://steamcommunity.com/tradeoffer/new/?partner=" + formats.ID32 + "&token=aB3-_xYz"
	if result.URL != want {
		t.Fatalf("url = %q, want %q", result.URL, want)
	}
	if fake.calls != 1 {
		t.Fatalf("made %d requests, want exactly 1", fake.calls)
	}
	if fake.credentials.SteamID != accountID || fake.credentials.AccessToken == "" {
		t.Fatalf("credentials = %+v", fake.credentials)
	}
	// The record carries no session id, so one has to be minted per request -
	// writing one back would rotate the vault generation and drop the capability.
	if fake.credentials.SessionID == "" {
		t.Fatal("no session id was minted, so the cookie session is incomplete")
	}
	if fake.credentials.IdentitySecret != "" || fake.credentials.DeviceID != "" {
		t.Fatal("authenticator secrets reached a request that does not sign anything")
	}
}

// The headline case for this feature: an account the vault holds a session for
// but no authenticator. It has the same access token a full record does, and
// this page needs nothing else.
func TestGetSteamTradeLinkWorksForALoginOnlyAccount(t *testing.T) {
	service, _, _ := newAuthServiceFixture(t)
	loginID := seedLoginOnlyRecordWithToken(t, service.vault, loginOnlySteamID, "session_only", "live-access-token")
	// Seeding wrote the vault, which rotated its generation, so the fixture's
	// capability is spent.
	grant := issueSensitiveGrant(t, service, loginID, "request-trade-link-login-only")
	service.confirmationClient = &fakeTradeLinkClient{body: tradeLinkPage(t, loginOnlySteamID, "Lm7Qz")}

	result, err := service.GetSteamTradeLink(loginID, grant.Capability)
	if err != nil {
		t.Fatalf("GetSteamTradeLink: %v", err)
	}
	if result.State != "ok" || result.URL == "" {
		t.Fatalf("result = %+v", result)
	}
}

// Steam serves the login page with a 200 in some flows. Reporting that as "this
// account has no trade URL" would send the user to Steam's settings page when
// what they need is to sign in again.
func TestGetSteamTradeLinkReportsASignedOutPageAsReauth(t *testing.T) {
	service, accountID, grant := newAuthServiceFixture(t)
	service.confirmationClient = &fakeTradeLinkClient{
		body: []byte(`<html><head><title>Sign In</title></head><body></body></html>`),
	}

	result, err := service.GetSteamTradeLink(accountID, grant.Capability)
	if err != nil {
		t.Fatalf("GetSteamTradeLink: %v", err)
	}
	if result.State != "reauth" || !result.NeedsLogin {
		t.Fatalf("result = %+v, want reauth", result)
	}
	if result.URL != "" {
		t.Fatalf("url = %q, want empty", result.URL)
	}
}

// A link for a different account is the one failure the user could not detect
// themselves: it looks exactly like a working trade URL until items never
// arrive. Nothing should reach the clipboard.
func TestGetSteamTradeLinkRefusesAnotherAccountsLink(t *testing.T) {
	service, accountID, grant := newAuthServiceFixture(t)
	service.confirmationClient = &fakeTradeLinkClient{body: tradeLinkPage(t, loginOnlySteamID, "aB3-_xYz")}

	result, err := service.GetSteamTradeLink(accountID, grant.Capability)
	if err != nil {
		t.Fatalf("GetSteamTradeLink: %v", err)
	}
	if result.State != "unavailable" || result.URL != "" {
		t.Fatalf("result = %+v, want unavailable with no url", result)
	}
}

func TestGetSteamTradeLinkClassifiesTransportFailures(t *testing.T) {
	cases := map[string]struct {
		err   error
		state string
	}{
		"rate limited": {&confirmationapi.Error{Kind: confirmationapi.FailureRateLimit,
			StatusCode: http.StatusTooManyRequests}, "rate-limit"},
		"offline":  {&confirmationapi.Error{Kind: confirmationapi.FailureOffline}, "offline"},
		"rejected": {&confirmationapi.Error{Kind: confirmationapi.FailureReauth}, "reauth"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			service, accountID, grant := newAuthServiceFixture(t)
			service.confirmationClient = &fakeTradeLinkClient{err: tc.err}

			result, err := service.GetSteamTradeLink(accountID, grant.Capability)
			if err != nil {
				t.Fatalf("GetSteamTradeLink: %v", err)
			}
			if result.State != tc.state {
				t.Fatalf("state = %q, want %q", result.State, tc.state)
			}
			if result.URL != "" {
				t.Fatalf("url = %q, want empty", result.URL)
			}
		})
	}
}

// A page that parsed but held nothing readable must not be reported as a
// failure the user can act on, nor as an empty-but-valid answer.
func TestGetSteamTradeLinkReportsAnUnreadablePageAsUnavailable(t *testing.T) {
	service, accountID, grant := newAuthServiceFixture(t)
	service.confirmationClient = &fakeTradeLinkClient{body: []byte(`<html><body>Nothing here</body></html>`)}

	result, err := service.GetSteamTradeLink(accountID, grant.Capability)
	if err != nil {
		t.Fatalf("GetSteamTradeLink: %v", err)
	}
	if result.State != "unavailable" || result.URL != "" {
		t.Fatalf("result = %+v", result)
	}
}

// The capability is the gate: without a valid one, nothing is read and no
// request leaves the machine.
func TestGetSteamTradeLinkRequiresACapability(t *testing.T) {
	service, accountID, _ := newAuthServiceFixture(t)
	fake := &fakeTradeLinkClient{body: tradeLinkPage(t, authServiceSteamID, "aB3-_xYz")}
	service.confirmationClient = fake

	if _, err := service.GetSteamTradeLink(accountID, "not-a-capability"); err == nil {
		t.Fatal("GetSteamTradeLink succeeded without a capability")
	}
	if fake.calls != 0 {
		t.Fatalf("made %d requests without a capability", fake.calls)
	}
}
