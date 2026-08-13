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

	headers := make(http.Header)
	headers.Set("User-Agent", MobileUserAgent)
	headers.Set("Cookie", webCookie(credentials))
	response, err := c.protocol.Do(ctx, protocol.Request{
		Method: http.MethodGet, Endpoint: parsed.String(), Route: protocol.RouteRequest,
		Header: headers, Timeout: RequestTimeout, MaxResponseBytes: maxTradeLinkBytes,
	})
	if err != nil {
		classified := classifyProtocolError(err)
		logTransportFailure("tradelink", classified)
		return nil, classified
	}
	return response.Body, nil
}
