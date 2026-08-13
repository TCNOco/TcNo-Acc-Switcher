package confirmationapi

import (
	"context"
	"net/http"
	"net/url"

	"TcNo-Acc-Switcher/internal/steamguard/protocol"
)

const (
	tradeLinkBase = "https://steamcommunity.com/profiles/"
	// A settings page for one account: a few KiB of form markup. The cap matches
	// the community pages next door, and is sized for the largest thing this can
	// actually receive, which is the signed-out login page.
	maxTradeLinkBytes = 512 << 10
)

// FetchTradeOfferPrivacyPage loads an account's trade-offer privacy page, the
// only place Steam shows an account its own trade URL.
//
// The method is fixed to GET and the path suffix is a literal, for the same
// reason gcpd.go hard-codes its app id - but with sharper teeth here. The same
// page answers a POST by ROTATING the trade token, which would silently break
// every trade link the user has already handed out to sites, bots and friends.
// There must be no request this package can be talked into making that has that
// effect.
func (c *Client) FetchTradeOfferPrivacyPage(ctx context.Context, credentials Credentials) ([]byte, error) {
	if c == nil || c.protocol == nil || ctx == nil || c.offline == nil {
		return nil, &Error{Kind: FailureInvalid}
	}
	if c.offline() {
		return nil, &Error{Kind: FailureOffline}
	}
	if err := validateWebCredentials(credentials); err != nil {
		return nil, err
	}
	parsed, err := url.Parse(tradeLinkBase + url.PathEscape(credentials.SteamID) + "/tradeoffers/privacy")
	if err != nil {
		return nil, &Error{Kind: FailureInvalid}
	}

	// Asked for as the desktop site, not as the Steam app. This is an account
	// settings page, and under the mobile shell Steam bounces it between forms
	// until the redirect budget runs out - which read, from the outside, as a
	// refused session on an account whose session was fine. The store page next
	// door is fetched the same way for the same reason.
	headers := make(http.Header)
	headers.Set("User-Agent", protocol.UserAgent)
	headers.Set("Cookie", desktopSessionCookie(credentials))
	// Steam canonicalises /profiles/<id64>/... to /id/<vanity>/... for any account
	// with a custom URL, so a large share of real accounts answer this with a 302
	// rather than the page. Following it needs the cookies, or the hop lands on
	// the login page and a perfectly good session reads as "sign in again" - which
	// is exactly what it did. Same host only; a redirect anywhere else is still
	// scrubbed, and a redirect to /login/ is followed and then recognised from the
	// body it returns.
	response, err := c.protocol.Do(ctx, protocol.Request{
		Method: http.MethodGet, Endpoint: parsed.String(), Route: protocol.RouteRequest,
		Header: headers, Timeout: RequestTimeout, MaxResponseBytes: maxTradeLinkBytes,
		AllowRedirects: true, PreserveHeadersOnRedirect: true,
	})
	if err != nil {
		classified := classifyProtocolError(err)
		logTransportFailure("tradelink", classified)
		return nil, classified
	}
	return response.Body, nil
}
