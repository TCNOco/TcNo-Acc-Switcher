package confirmationapi

import (
	"context"
	"net/http"

	"TcNo-Acc-Switcher/internal/steamguard/protocol"
)

const (
	// The CS2 store page. Personalised by cookie: signed in, each purchase
	// section says whether the account already owns that package, which is the
	// only place Prime ownership is visible to a web request. GCPD's own
	// "primeaccount" tab is empty even for a Prime account, and the licenses page
	// serves a table-less responsive shell to this client.
	cs2StorePageURL = "https://store.steampowered.com/app/730/CounterStrike_2/"

	// A store app page carries the full storefront chrome and every purchase
	// section, so it is far larger than a GCPD tab.
	maxStorePageBytes = 4 << 20
)

// FetchCS2StorePage loads the CS2 store page as this account sees it.
//
// The response is HTML; reading the Prime flag out of it lives in the
// primestatus package so the two can be tested apart.
func (c *Client) FetchCS2StorePage(ctx context.Context, credentials Credentials) ([]byte, error) {
	if c == nil || c.protocol == nil || ctx == nil || c.offline == nil {
		return nil, &Error{Kind: FailureInvalid}
	}
	if c.offline() {
		return nil, &Error{Kind: FailureOffline}
	}
	if err := validateWebCredentials(credentials); err != nil {
		return nil, err
	}

	headers := make(http.Header)
	headers.Set("User-Agent", protocol.UserAgent)
	headers.Set("Cookie", storeCookie(credentials))
	// RouteTransfer, not RouteRequest: the route selects a fixed host allowlist,
	// and store.steampowered.com is only on the transfer one.
	response, err := c.protocol.Do(ctx, protocol.Request{
		Method: http.MethodGet, Endpoint: cs2StorePageURL, Route: protocol.RouteTransfer,
		Header: headers, Timeout: RequestTimeout, MaxResponseBytes: maxStorePageBytes,
	})
	if err != nil {
		classified := classifyProtocolError(err)
		logTransportFailure("storepage", classified)
		return nil, classified
	}
	return response.Body, nil
}

// storeCookie is the session without the mobile client markers.
//
// mobileClient=android asks for Steam's mobile shell. The language pin is kept
// for the same reason GCPD keeps it, though the Prime flag itself is matched on
// a CSS class rather than on translated text.
func storeCookie(credentials Credentials) string {
	return desktopSessionCookie(credentials) +
		// Without a birth date the store interposes an age gate on a mature-rated
		// app and serves none of the purchase sections.
		"; birthtime=283996801; lastagecheckage=1-January-1979"
}
