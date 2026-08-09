package confirmationapi

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestFetchOwnedAppsBuildsTheExactRequest(t *testing.T) {
	transport := &queuedTransport{responses: []*http.Response{
		jsonResponse(http.StatusOK, `{"response":{"game_count":2,"games":[{"appid":10},{"appid":70}]}}`),
	}}
	client := testClient(transport, func() bool { return false })

	appIDs, err := client.FetchOwnedApps(context.Background(), testCredentials())
	if err != nil {
		t.Fatalf("FetchOwnedApps: %v", err)
	}
	if len(appIDs) != 2 || appIDs[0] != 10 || appIDs[1] != 70 {
		t.Fatalf("appIDs = %v", appIDs)
	}

	request := transport.requests[0]
	if request.Method != http.MethodGet || request.URL.Host != "api.steampowered.com" ||
		request.URL.Path != "/IPlayerService/GetOwnedGames/v1" {
		t.Fatalf("request = %s %s", request.Method, request.URL)
	}
	query := request.URL.Query()
	if query.Get("steamid") != testCredentials().SteamID {
		t.Fatalf("steamid = %q", query.Get("steamid"))
	}
	// This call authenticates by token in the query, not by cookie - the opposite
	// of every other request in this package.
	if query.Get("access_token") != testCredentials().AccessToken {
		t.Fatal("access token missing from the query")
	}
	if request.Header.Get("Cookie") != "" {
		t.Fatalf("unexpected Cookie header: %q", request.Header.Get("Cookie"))
	}
	// include_appinfo would return a name per app; names come from the app id ->
	// name map internal/steam already maintains.
	if query.Get("include_appinfo") != "" {
		t.Fatal("include_appinfo was requested")
	}
}

func TestFetchOwnedAppsAcceptsCredentialsWithoutASessionID(t *testing.T) {
	// The narrower validator's whole point: no cookie session is opened, so a
	// record with no session id must not be locked out.
	credentials := testCredentials()
	credentials.SessionID = ""
	credentials.DeviceID = ""
	credentials.IdentitySecret = ""

	transport := &queuedTransport{responses: []*http.Response{
		jsonResponse(http.StatusOK, `{"response":{"game_count":1,"games":[{"appid":10}]}}`),
	}}
	client := testClient(transport, func() bool { return false })
	if _, err := client.FetchOwnedApps(context.Background(), credentials); err != nil {
		t.Fatalf("FetchOwnedApps: %v", err)
	}
}

// The failure that matters: Steam answers an unauthorised caller with HTTP 200
// and an empty response object, which is the same shape an account owning
// nothing would produce. Reading it as an empty library would cache a wrong
// answer indistinguishable from a right one.
func TestParseOwnedAppsTreatsAnEmptyResponseAsReauth(t *testing.T) {
	for name, body := range map[string]string{
		"empty response object": `{"response":{}}`,
		"empty games array":     `{"response":{"game_count":0,"games":[]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseOwnedApps([]byte(body))
			var apiErr *Error
			if !errors.As(err, &apiErr) || apiErr.Kind != FailureReauth {
				t.Fatalf("err = %v, want FailureReauth", err)
			}
		})
	}
}

func TestParseOwnedAppsRejectsAMalformedBody(t *testing.T) {
	if _, err := ParseOwnedApps([]byte("<html>not json</html>")); err == nil {
		t.Fatal("ParseOwnedApps accepted a non-JSON body")
	}
}
