package confirmationapi

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/protocol"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type queuedTransport struct {
	mu        sync.Mutex
	responses []*http.Response
	requests  []*http.Request
	err       error
}

func (t *queuedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.requests = append(t.requests, request.Clone(request.Context()))
	if t.err != nil {
		return nil, t.err
	}
	if len(t.responses) == 0 {
		return nil, errors.New("unexpected request")
	}
	response := t.responses[0]
	t.responses = t.responses[1:]
	response.Request = request
	return response, nil
}

func jsonResponse(status int, body string, headers ...http.Header) *http.Response {
	header := make(http.Header)
	if len(headers) > 0 {
		header = headers[0]
	}
	return &http.Response{
		StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)),
	}
}

func testCredentials() Credentials {
	return Credentials{
		SteamID: "76561198000000000", DeviceID: "android:01234567-89ab-cdef-0123-456789abcdef",
		IdentitySecret: "AAECAwQFBgcICQoLDA0ODxAREhM=",
		AccessToken:    "eyJhbGciOiJIUzI1NiJ9.payload.signature", SessionID: "0123456789ABCDEF0123456789ABCDEF",
	}
}

func testClient(transport http.RoundTripper, offline func() bool) *Client {
	return NewClient(Options{
		Protocol: protocol.NewClient(protocol.Options{Transport: transport}),
		Clock:    fixedClock{now: time.Unix(1_700_000_000, 0)}, Offline: offline,
	})
}

const firstList = `{"success":true,"message":"","needauth":false,"conf":[{"id":"100","nonce":"200","creator_id":"300","headline":"<b>Trade offer</b><script>bad()</script>","summary":["One &amp; two"],"accept":"Accept","cancel":"Deny","icon":"https://example.invalid/icon.png","type":"2"},{"id":"101","nonce":"201","creator_id":"301","headline":"Market item","summary":[],"accept":"List","cancel":"Cancel","icon":"","type":3}]}`

func TestGenerateConfirmationHashVectorAndTagTruncation(t *testing.T) {
	hash, err := GenerateConfirmationHash(testCredentials().IdentitySecret, 1_700_000_000, "conf")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "MnyTnNQlGkbWQN0NCU9mCTxb/Ec=" {
		t.Fatalf("hash = %q", hash)
	}
	longHash, err := GenerateConfirmationHash(testCredentials().IdentitySecret, 1_700_000_000, strings.Repeat("x", 40))
	if err != nil {
		t.Fatal(err)
	}
	truncatedHash, _ := GenerateConfirmationHash(testCredentials().IdentitySecret, 1_700_000_000, strings.Repeat("x", 32))
	if longHash != truncatedHash {
		t.Fatal("tag was not truncated to Steam's 32-byte limit")
	}
}

func TestListExactRequestAndHTMLSafeDTO(t *testing.T) {
	transport := &queuedTransport{responses: []*http.Response{jsonResponse(http.StatusOK, firstList)}}
	client := testClient(transport, func() bool { return false })
	confirmations, err := client.List(context.Background(), testCredentials())
	if err != nil {
		t.Fatal(err)
	}
	if len(confirmations) != 2 {
		t.Fatalf("confirmations = %d", len(confirmations))
	}
	item := confirmations[0].Item()
	if item.Title != "Trade offer" || len(item.Summary) != 1 || item.Summary[0] != "One & two" || strings.Contains(item.Title, "bad") {
		t.Fatalf("unsafe DTO: %#v", item)
	}
	request := transport.requests[0]
	wantURL := "https://steamcommunity.com/mobileconf/getlist?a=76561198000000000&k=MnyTnNQlGkbWQN0NCU9mCTxb%2FEc%3D&m=react&p=android%3A01234567-89ab-cdef-0123-456789abcdef&t=1700000000&tag=conf"
	if request.Method != http.MethodGet || request.URL.String() != wantURL {
		t.Fatalf("request = %s %s", request.Method, request.URL)
	}
	wantCookie := "steamLoginSecure=76561198000000000%7C%7CeyJhbGciOiJIUzI1NiJ9.payload.signature; sessionid=0123456789ABCDEF0123456789ABCDEF; mobileClient=android; mobileClientVersion=777777%203.6.4"
	if request.Header.Get("Cookie") != wantCookie || request.Header.Get("Authorization") != "" {
		t.Fatalf("unexpected authentication headers: %#v", request.Header)
	}
	if strings.Contains(request.URL.String(), testCredentials().AccessToken) {
		t.Fatal("access token leaked into URL")
	}
}

func TestListSnapshotsHaveStableHandlesAndNoStaleRows(t *testing.T) {
	secondList := `{"success":true,"message":"","needauth":false,"conf":[{"id":100,"nonce":999,"creator_id":300,"headline":"Updated trade","summary":[],"accept":"Accept","cancel":"Deny","icon":"","type":2}]}`
	transport := &queuedTransport{responses: []*http.Response{
		jsonResponse(http.StatusOK, firstList), jsonResponse(http.StatusOK, secondList),
	}}
	client := testClient(transport, func() bool { return false })
	first, err := client.List(context.Background(), testCredentials())
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.List(context.Background(), testCredentials())
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || first[0].Item().Handle != second[0].Item().Handle || second[0].Item().Title != "Updated trade" {
		t.Fatalf("snapshots were not reconciliable: first=%#v second=%#v", first[0].Item(), second)
	}
}

func TestDetailsAndDecisionsUseExactRoutesAndTags(t *testing.T) {
	detailsBody := `{"success":true,"needauth":false,"message":"","html":"<h2>Trade offer</h2><table><tr><th>You receive</th><td>Item &amp; one</td></tr></table><script>secret()</script>"}`
	transport := &queuedTransport{responses: []*http.Response{
		jsonResponse(http.StatusOK, firstList), jsonResponse(http.StatusOK, detailsBody),
		jsonResponse(http.StatusOK, `{"success":true,"needauth":false,"message":""}`),
		jsonResponse(http.StatusOK, `{"success":true,"needauth":false,"message":""}`),
	}}
	client := testClient(transport, func() bool { return false })
	confirmations, err := client.List(context.Background(), testCredentials())
	if err != nil {
		t.Fatal(err)
	}
	details, err := client.FetchDetails(context.Background(), testCredentials(), confirmations[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(details.Fields) != 2 || details.Fields[1] != (TextField{Label: "You receive", Value: "Item & one"}) {
		t.Fatalf("details = %#v", details.Fields)
	}
	for _, field := range details.Fields {
		if strings.Contains(field.Value, "secret") || strings.ContainsAny(field.Value, "<>") {
			t.Fatalf("raw HTML reached DTO: %#v", field)
		}
	}
	if err := client.Decide(context.Background(), testCredentials(), confirmations[0], Allow); err != nil {
		t.Fatal(err)
	}
	if err := client.Decide(context.Background(), testCredentials(), confirmations[0], Deny); err != nil {
		t.Fatal(err)
	}
	// l=english is pinned so the listing parser is not handed the account's own
	// language; the signature covers the tag and the time, not the query.
	wantDetails := "https://steamcommunity.com/mobileconf/details/100?a=76561198000000000&k=UrC0%2BPlqKtzPgfCqGpxK1BQXtnc%3D&l=english&m=react&p=android%3A01234567-89ab-cdef-0123-456789abcdef&t=1700000000&tag=details"
	wantAllow := "https://steamcommunity.com/mobileconf/ajaxop?a=76561198000000000&cid=100&ck=200&k=0bBIomcF2qVl%2FzF4isGPy7YSJTs%3D&m=react&op=allow&p=android%3A01234567-89ab-cdef-0123-456789abcdef&t=1700000000&tag=accept"
	wantDeny := "https://steamcommunity.com/mobileconf/ajaxop?a=76561198000000000&cid=100&ck=200&k=0E4Gy%2Fo%2FDiuW%2FfzukIVNb6S6rAg%3D&m=react&op=cancel&p=android%3A01234567-89ab-cdef-0123-456789abcdef&t=1700000000&tag=reject"
	for index, want := range []string{wantDetails, wantAllow, wantDeny} {
		if got := transport.requests[index+1].URL.String(); got != want {
			t.Fatalf("request %d = %s, want %s", index, got, want)
		}
	}
}

// A multi-select must be one signed request: the signature is built from the
// tag and the current second, so two single decisions fired back to back sign
// identically and Steam rejects the second as a replay.
func TestDecideBatchPostsOneSignedMultiRequest(t *testing.T) {
	transport := &queuedTransport{responses: []*http.Response{
		jsonResponse(http.StatusOK, firstList),
		jsonResponse(http.StatusOK, `{"success":true,"needauth":false,"message":""}`),
	}}
	client := testClient(transport, func() bool { return false })
	confirmations, err := client.List(context.Background(), testCredentials())
	if err != nil {
		t.Fatal(err)
	}
	if err := client.DecideBatch(context.Background(), testCredentials(), confirmations, Deny); err != nil {
		t.Fatal(err)
	}
	request := transport.requests[1]
	if request.Method != http.MethodPost || request.URL.String() != MultiDecisionEndpoint {
		t.Fatalf("request = %s %s", request.Method, request.URL)
	}
	body, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatal(err)
	}
	wantBody := "a=76561198000000000&cid%5B%5D=100&cid%5B%5D=101&ck%5B%5D=200&ck%5B%5D=201" +
		"&k=0E4Gy%2Fo%2FDiuW%2FfzukIVNb6S6rAg%3D&m=react&op=cancel" +
		"&p=android%3A01234567-89ab-cdef-0123-456789abcdef&t=1700000000&tag=reject"
	if string(body) != wantBody {
		t.Fatalf("body = %s, want %s", body, wantBody)
	}
	if got := request.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
		t.Fatalf("content type = %q", got)
	}
}

func TestDecideBatchOfOneUsesTheSingleEndpoint(t *testing.T) {
	transport := &queuedTransport{responses: []*http.Response{
		jsonResponse(http.StatusOK, firstList),
		jsonResponse(http.StatusOK, `{"success":true,"needauth":false,"message":""}`),
	}}
	client := testClient(transport, func() bool { return false })
	confirmations, err := client.List(context.Background(), testCredentials())
	if err != nil {
		t.Fatal(err)
	}
	if err := client.DecideBatch(context.Background(), testCredentials(), confirmations[:1], Allow); err != nil {
		t.Fatal(err)
	}
	request := transport.requests[1]
	if request.Method != http.MethodGet || !strings.HasPrefix(request.URL.String(), DecisionEndpoint+"?") {
		t.Fatalf("request = %s %s", request.Method, request.URL)
	}
}

func TestTypedFailureStatesAndTokenRedaction(t *testing.T) {
	t.Run("offline", func(t *testing.T) {
		transport := &queuedTransport{}
		_, err := testClient(transport, func() bool { return true }).List(context.Background(), testCredentials())
		assertFailure(t, err, FailureOffline)
		if len(transport.requests) != 0 {
			t.Fatal("offline request reached transport")
		}
	})
	t.Run("reauth status", func(t *testing.T) {
		transport := &queuedTransport{responses: []*http.Response{jsonResponse(http.StatusUnauthorized, "denied")}}
		_, err := testClient(transport, func() bool { return false }).List(context.Background(), testCredentials())
		assertFailure(t, err, FailureReauth)
	})
	t.Run("rate limit", func(t *testing.T) {
		header := http.Header{"Retry-After": {"30"}}
		transport := &queuedTransport{responses: []*http.Response{jsonResponse(http.StatusTooManyRequests, "slow", header)}}
		_, err := testClient(transport, func() bool { return false }).List(context.Background(), testCredentials())
		failure := assertFailure(t, err, FailureRateLimit)
		if !failure.HasRetryAfter || failure.RetryAfter != 30*time.Second {
			t.Fatalf("retry metadata = %#v", failure)
		}
	})
	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		transport := &queuedTransport{err: context.Canceled}
		_, err := testClient(transport, func() bool { return false }).List(ctx, testCredentials())
		assertFailure(t, err, FailureCanceled)
	})
	t.Run("timeout", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer cancel()
		transport := &queuedTransport{err: context.DeadlineExceeded}
		_, err := testClient(transport, func() bool { return false }).List(ctx, testCredentials())
		assertFailure(t, err, FailureTimeout)
	})
	t.Run("login redirect", func(t *testing.T) {
		header := http.Header{"Location": {"https://steamcommunity.com/login/home/"}}
		transport := &queuedTransport{responses: []*http.Response{jsonResponse(http.StatusFound, "", header)}}
		_, err := testClient(transport, func() bool { return false }).List(context.Background(), testCredentials())
		assertFailure(t, err, FailureReauth)
	})
	t.Run("need auth body", func(t *testing.T) {
		transport := &queuedTransport{responses: []*http.Response{jsonResponse(http.StatusOK, `{"success":false,"message":"token redacted","needauth":true,"conf":[]}`)}}
		_, err := testClient(transport, func() bool { return false }).List(context.Background(), testCredentials())
		assertFailure(t, err, FailureReauth)
		if strings.Contains(err.Error(), testCredentials().AccessToken) || strings.Contains(err.Error(), "token redacted") {
			t.Fatalf("response or credential leaked in error: %v", err)
		}
	})
}

func TestRejectsMalformedAndDuplicateResponses(t *testing.T) {
	duplicate := `{"success":true,"message":"","needauth":false,"conf":[{"id":"1","nonce":"2","creator_id":"3","headline":"a","summary":[],"accept":"a","cancel":"d","icon":"","type":2},{"id":1,"nonce":2,"creator_id":4,"headline":"b","summary":[],"accept":"a","cancel":"d","icon":"","type":2}]}`
	for _, body := range []string{
		duplicate,
		`{"success":true,"needauth":false,"conf":[]} trailing`,
	} {
		transport := &queuedTransport{responses: []*http.Response{jsonResponse(http.StatusOK, body)}}
		_, err := testClient(transport, func() bool { return false }).List(context.Background(), testCredentials())
		assertFailure(t, err, FailureFailed)
	}
}

// livePendingList mirrors a real getlist response with one pending trade,
// including the fields Steam sends beyond the ones we consume.
const livePendingList = `{"success":true,"conf":[{"type":2,"type_name":"Trade Offer","id":"14412583061",` +
	`"creator_id":"7681695590","nonce":"10056326836841689505","creation_time":1753380000,"cancel":"Cancel",` +
	`"accept":"Confirm","icon":"https://avatars.akamai.steamstatic.com/abc_full.jpg","multi":false,` +
	`"headline":"tradepartner","summary":["You will give up 1 item"],"warn":null}]}`

func TestListAcceptsLivePendingItemShape(t *testing.T) {
	transport := &queuedTransport{responses: []*http.Response{jsonResponse(http.StatusOK, livePendingList)}}
	confirmations, err := testClient(transport, func() bool { return false }).List(context.Background(), testCredentials())
	if err != nil {
		t.Fatal(err)
	}
	if len(confirmations) != 1 {
		t.Fatalf("confirmations = %d", len(confirmations))
	}
	item := confirmations[0].Item()
	if item.TypeName != "Trade Offer" || item.Type != 2 || item.Title != "tradepartner" ||
		item.CreationTime != 1753380000 || item.Icon != "https://avatars.akamai.steamstatic.com/abc_full.jpg" {
		t.Fatalf("item = %#v", item)
	}
	if len(item.Summary) != 1 || item.Summary[0] != "You will give up 1 item" {
		t.Fatalf("summary = %#v", item.Summary)
	}
}

// Steam sizes the list icon for its own phone row, which is smaller than the
// tile the window draws. The avatar in the test above covers the other half of
// this: a URL with no size segment has to survive untouched.
func TestListAsksForAnItemIconLargeEnoughForTheRow(t *testing.T) {
	body := `{"success":true,"conf":[{"type":3,"type_name":"Market Listing","id":"1","creator_id":"2",` +
		`"nonce":"3","creation_time":1753380000,"headline":"Wanderer","summary":[],` +
		`"icon":"https://community.fastly.steamstatic.com/economy/image/abc/73fx73f"}]}`
	transport := &queuedTransport{responses: []*http.Response{jsonResponse(http.StatusOK, body)}}
	confirmations, err := testClient(transport, func() bool { return false }).List(context.Background(), testCredentials())
	if err != nil {
		t.Fatal(err)
	}
	want := "https://community.fastly.steamstatic.com/economy/image/abc/" + economyImageSize
	if got := confirmations[0].Item().Icon; got != want {
		t.Fatalf("icon = %q, want %q", got, want)
	}
}

func TestMobileConfirmationRequestMatchesTheSteamMobileApp(t *testing.T) {
	transport := &queuedTransport{responses: []*http.Response{jsonResponse(http.StatusOK, livePendingList)}}
	if _, err := testClient(transport, func() bool { return false }).List(context.Background(), testCredentials()); err != nil {
		t.Fatal(err)
	}
	header := transport.requests[0].Header
	if header.Get("User-Agent") != MobileUserAgent {
		t.Fatalf("user agent = %q", header.Get("User-Agent"))
	}
	for _, name := range []string{"Accept", "Origin", "Referer", "X-Requested-With"} {
		if header.Get(name) != "" {
			t.Fatalf("%s must not be sent: %q", name, header.Get(name))
		}
	}
}

func TestSignatureUsesInjectedClock(t *testing.T) {
	transport := &queuedTransport{responses: []*http.Response{jsonResponse(http.StatusOK, livePendingList)}}
	client := NewClient(Options{
		Protocol: protocol.NewClient(protocol.Options{Transport: transport}),
		Clock:    fixedClock{now: time.Unix(1_700_000_500, 0)}, Offline: func() bool { return false },
	})
	if _, err := client.List(context.Background(), testCredentials()); err != nil {
		t.Fatal(err)
	}
	query := transport.requests[0].URL.Query()
	if query.Get("t") != "1700000500" {
		t.Fatalf("t = %q", query.Get("t"))
	}
	wantHash, err := GenerateConfirmationHash(testCredentials().IdentitySecret, 1_700_000_500, "conf")
	if err != nil {
		t.Fatal(err)
	}
	if query.Get("k") != wantHash {
		t.Fatalf("k = %q, want %q", query.Get("k"), wantHash)
	}
}

// success:false without needauth is Steam declining the signed request — a
// session verdict the probe reports as needing a sign-in, distinct from a
// response that merely could not be decoded.
func TestListRefusalIsItsOwnFailureKind(t *testing.T) {
	body := `{"success":false,"needauth":false,"message":"Oh nooo, we were unable to load your confirmations.","conf":[]}`
	transport := &queuedTransport{responses: []*http.Response{jsonResponse(http.StatusOK, body)}}
	_, err := testClient(transport, func() bool { return false }).List(context.Background(), testCredentials())
	assertFailure(t, err, FailureRefused)

	malformed := &queuedTransport{responses: []*http.Response{jsonResponse(http.StatusOK, "<html>not json</html>")}}
	_, err = testClient(malformed, func() bool { return false }).List(context.Background(), testCredentials())
	assertFailure(t, err, FailureFailed)
}

// Steam adds confirmation kinds without notice; one unknown type must render
// as a generic confirmation, not fail the whole list.
func TestListAcceptsUnknownConfirmationType(t *testing.T) {
	body := `{"success":true,"needauth":false,"conf":[{"id":"7","nonce":"8","creator_id":"9",` +
		`"type":9,"headline":"New API key request","summary":["Review this request"],"accept":"Approve","cancel":"Deny","icon":""}]}`
	transport := &queuedTransport{responses: []*http.Response{jsonResponse(http.StatusOK, body)}}
	confirmations, err := testClient(transport, func() bool { return false }).List(context.Background(), testCredentials())
	if err != nil {
		t.Fatal(err)
	}
	item := confirmations[0].Item()
	if item.Type != 9 || item.TypeLabel != "Confirmation" || item.Title != "New API key request" {
		t.Fatalf("item = %#v", item)
	}
}

// A dead session comes back as an HTML page whose links use the
// steammobile://lostauth scheme rather than as a JSON envelope; it has to read
// as reauth or the window and the session probe both call it a generic error.
func TestListLostAuthPageReadsAsReauth(t *testing.T) {
	body := `<html><body><a href="steammobile://lostauth">Please log in</a></body></html>`
	transport := &queuedTransport{responses: []*http.Response{jsonResponse(http.StatusOK, body)}}
	_, err := testClient(transport, func() bool { return false }).List(context.Background(), testCredentials())
	assertFailure(t, err, FailureReauth)
}

func assertFailure(t *testing.T, err error, kind FailureKind) *Error {
	t.Helper()
	var failure *Error
	if !errors.As(err, &failure) || failure.Kind != kind {
		t.Fatalf("failure = %#v, want %s", err, kind)
	}
	return failure
}

func TestIdentitySecretFixtureIsTwentyBytes(t *testing.T) {
	decoded, err := base64.StdEncoding.Strict().DecodeString(testCredentials().IdentitySecret)
	if err != nil || len(decoded) != 20 {
		t.Fatalf("invalid fixture: %v, %d", err, len(decoded))
	}
}

func TestCredentialFormattingIsRedacted(t *testing.T) {
	credentials := testCredentials()
	formatted := fmt.Sprintf("%v %#v", credentials, credentials)
	if strings.Contains(formatted, credentials.AccessToken) || strings.Contains(formatted, credentials.IdentitySecret) {
		t.Fatalf("credential formatter leaked a secret: %s", formatted)
	}
}

// The CS2 endpoints are unsigned GETs added after the confirmation calls, so
// they do not share their code path and could lose the offline gate without any
// other test noticing. Offline is a user promise that the app makes no network
// requests, and the store page is the only one that leaves steamcommunity.com.
func TestCS2RequestsAreRefusedOffline(t *testing.T) {
	for name, call := range map[string]func(*Client) ([]byte, error){
		"gcpd": func(c *Client) ([]byte, error) {
			return c.FetchCS2GCPD(context.Background(), testCredentials())
		},
		"store page": func(c *Client) ([]byte, error) {
			return c.FetchCS2StorePage(context.Background(), testCredentials())
		},
	} {
		t.Run(name, func(t *testing.T) {
			transport := &queuedTransport{}
			_, err := call(testClient(transport, func() bool { return true }))
			assertFailure(t, err, FailureOffline)
			if len(transport.requests) != 0 {
				t.Fatal("offline request reached transport")
			}
		})
	}
}
