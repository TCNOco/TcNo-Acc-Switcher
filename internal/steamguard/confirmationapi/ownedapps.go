package confirmationapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/protocol"
)

const (
	ownedGamesEndpoint = "https://api.steampowered.com/IPlayerService/GetOwnedGames/v1"
	// A library of a few thousand apps is a few hundred KiB of ids. The cap is
	// generous because the alternative - a truncated body - parses as a shorter
	// library rather than as an error.
	maxOwnedGamesBytes = 4 << 20
	// Longer than RequestTimeout: this is the one call whose response grows with
	// the size of the account, and 15s is not enough for a large library on a
	// slow link.
	ownedGamesTimeout = 45 * time.Second
)

// FetchOwnedApps lists the app ids an account owns.
//
// Unlike the other calls here this one is not a cookie session - Steam accepts
// the vault's access token directly as a query parameter, on a host already
// permitted for requests, so it needs neither a SessionID nor mobileconf's
// signing inputs. See ownedapps_spike_test.go in the parent package for the
// live probe that established this.
func (c *Client) FetchOwnedApps(ctx context.Context, credentials Credentials) ([]uint32, error) {
	if c == nil || c.protocol == nil || ctx == nil || c.offline == nil {
		return nil, &Error{Kind: FailureInvalid}
	}
	if c.offline() {
		return nil, &Error{Kind: FailureOffline}
	}
	if err := validateOwnedAppsCredentials(credentials); err != nil {
		return nil, err
	}

	query := url.Values{
		"access_token": {credentials.AccessToken},
		"steamid":      {credentials.SteamID},
		// Names are resolved from the app id -> name map internal/steam already
		// maintains, so include_appinfo would only bloat the response.
		"include_played_free_games": {"1"},
		"include_free_sub":          {"1"},
		"skip_unvetted_apps":        {"0"},
	}

	response, err := c.protocol.Do(ctx, protocol.Request{
		Method: http.MethodGet, Endpoint: ownedGamesEndpoint + "?" + query.Encode(),
		Route: protocol.RouteRequest, Timeout: ownedGamesTimeout, MaxResponseBytes: maxOwnedGamesBytes,
	})
	if err != nil {
		classified := classifyProtocolError(err)
		logTransportFailure("ownedapps", classified)
		return nil, classified
	}
	return ParseOwnedApps(response.Body)
}

// ParseOwnedApps reads the app ids out of a GetOwnedGames response.
//
// An empty response object is reported as FailureReauth, not as an empty
// library. Steam answers a caller it will not authorise with HTTP 200 and
// {"response":{}} - the same shape a genuinely empty account would produce -
// so the only safe reading is that the token was not accepted. Treating it as
// "owns nothing" would cache a wrong answer that looks indistinguishable from
// a correct one.
func ParseOwnedApps(body []byte) ([]uint32, error) {
	var payload struct {
		Response struct {
			GameCount int `json:"game_count"`
			Games     []struct {
				AppID uint32 `json:"appid"`
			} `json:"games"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, &Error{Kind: FailureFailed}
	}
	if len(payload.Response.Games) == 0 {
		return nil, &Error{Kind: FailureReauth}
	}
	appIDs := make([]uint32, 0, len(payload.Response.Games))
	for _, game := range payload.Response.Games {
		if game.AppID == 0 {
			continue
		}
		appIDs = append(appIDs, game.AppID)
	}
	if len(appIDs) == 0 {
		return nil, &Error{Kind: FailureFailed}
	}
	return appIDs, nil
}

// validateOwnedAppsCredentials checks only what a token-authenticated API call
// needs.
//
// validateWebCredentials additionally demands a SessionID, which belongs to a
// cookie session this request does not open. Requiring one would lock the call
// out of accounts whose stored record has no session id.
func validateOwnedAppsCredentials(credentials Credentials) error {
	steamID, err := strconv.ParseUint(credentials.SteamID, 10, 64)
	if err != nil || steamID == 0 || strconv.FormatUint(steamID, 10) != credentials.SteamID {
		return &Error{Kind: FailureInvalid}
	}
	if len(credentials.AccessToken) < 16 || len(credentials.AccessToken) > 4096 || !safeToken(credentials.AccessToken, false) {
		return &Error{Kind: FailureInvalid}
	}
	return nil
}
