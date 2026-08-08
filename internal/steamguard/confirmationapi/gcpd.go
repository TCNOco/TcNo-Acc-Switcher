package confirmationapi

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"TcNo-Acc-Switcher/internal/steamguard/protocol"
)

const (
	gcpdBase = "https://steamcommunity.com/profiles/"
	// cs2AppID is hard-coded: this endpoint is only used for CS2 cooldowns, and
	// letting a caller pass an app id would widen it into a general scraper.
	cs2AppID     = "730"
	maxGCPDBytes = 512 << 10
)

// GCPD tabs. The matchmaking tab carries the cooldown table AND the Premier and
// Wingman ranks; accountmain carries the CS2 in-game profile level and XP, which
// is not a matchmaking rank, and is not read in production.
const (
	GCPDTabMatchmaking = "matchmaking"
	GCPDTabAccountMain = "accountmain"
	// GCPDTabPrimeAccount is a dedicated page rather than something to infer.
	// Not read in production: it would be a second request per account, and the
	// matchmaking tab already carries everything the sweep needs.
	GCPDTabPrimeAccount = "primeaccount"
)

// FetchCS2GCPD loads an account's CS2 matchmaking "Game Personal Data" page.
//
// This is the only place Valve exposes a competitive matchmaking cooldown, and
// only to the account itself. The response is HTML; parsing lives in the gcpd
// package so the two can be tested apart.
func (c *Client) FetchCS2GCPD(ctx context.Context, credentials Credentials) ([]byte, error) {
	return c.FetchCS2GCPDTab(ctx, credentials, GCPDTabMatchmaking)
}

// FetchCS2GCPDTab loads one tab of the CS2 GCPD page.
//
// Kept separate from FetchCS2GCPD so the sweep's interface stays single-argument
// and the tab cannot be varied by accident: a caller that fetches a different
// tab must also bring its own parser, since gcpd.Parse's markers match the
// matchmaking page and would read another tab as "no cooldown".
func (c *Client) FetchCS2GCPDTab(ctx context.Context, credentials Credentials, tab string) ([]byte, error) {
	if c == nil || c.protocol == nil || ctx == nil || c.offline == nil {
		return nil, &Error{Kind: FailureInvalid}
	}
	if c.offline() {
		return nil, &Error{Kind: FailureOffline}
	}
	if tab != GCPDTabMatchmaking && tab != GCPDTabAccountMain && tab != GCPDTabPrimeAccount {
		return nil, &Error{Kind: FailureInvalid}
	}
	if err := validateWebCredentials(credentials); err != nil {
		return nil, err
	}
	parsed, err := url.Parse(gcpdBase + url.PathEscape(credentials.SteamID) + "/gcpd/" + cs2AppID)
	if err != nil {
		return nil, &Error{Kind: FailureInvalid}
	}
	parsed.RawQuery = url.Values{"tab": {tab}}.Encode()

	headers := make(http.Header)
	headers.Set("User-Agent", MobileUserAgent)
	headers.Set("Cookie", webCookie(credentials))
	response, err := c.protocol.Do(ctx, protocol.Request{
		Method: http.MethodGet, Endpoint: parsed.String(), Route: protocol.RouteRequest,
		Header: headers, Timeout: RequestTimeout, MaxResponseBytes: maxGCPDBytes,
	})
	if err != nil {
		classified := classifyProtocolError(err)
		logTransportFailure("gcpd", classified)
		return nil, classified
	}
	return response.Body, nil
}

// webCookie adds the language pin to the mobileconf cookie set.
//
// Steam_Language is load-bearing rather than cosmetic: the parser identifies the
// cooldown table by its English header, and a localised render would look to it
// like an account with no cooldown.
func webCookie(credentials Credentials) string {
	return confirmationCookie(credentials) + "; Steam_Language=english"
}

// validateWebCredentials checks only what an unsigned community GET needs.
//
// validateCredentials additionally demands a DeviceID and an IdentitySecret,
// but both are mobileconf *signing* inputs - they never reach an unsigned
// request. A login-only record has neither, so requiring them here would lock
// this endpoint to accounts with a full authenticator.
func validateWebCredentials(credentials Credentials) error {
	steamID, err := strconv.ParseUint(credentials.SteamID, 10, 64)
	if err != nil || steamID == 0 || strconv.FormatUint(steamID, 10) != credentials.SteamID {
		return &Error{Kind: FailureInvalid}
	}
	if len(credentials.AccessToken) < 16 || len(credentials.AccessToken) > 4096 || !safeToken(credentials.AccessToken, false) {
		return &Error{Kind: FailureInvalid}
	}
	if len(credentials.SessionID) < 16 || len(credentials.SessionID) > 64 || !isASCIIHex(credentials.SessionID) {
		return &Error{Kind: FailureInvalid}
	}
	return nil
}
