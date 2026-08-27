package steamguard

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/capability"
	"TcNo-Acc-Switcher/internal/steamguard/confirmationapi"
	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/otp"
	"TcNo-Acc-Switcher/internal/steamguard/protocol"
)

func TestConfirmationServiceKeepsPrivateKeysNative(t *testing.T) {
	service, _ := setupQRService(t)
	transport := &confirmationQueueTransport{responses: []*http.Response{
		confirmationHTTPResponse(http.StatusOK, `{"success":true,"message":"","needauth":false,"conf":[{"id":"100","nonce":"200","creator_id":"300","headline":"<b>Trade offer</b><script>bad()</script>","summary":["One &amp; two"],"accept":"Accept","cancel":"Deny","icon":"","type":"2"}]}`),
		confirmationHTTPResponse(http.StatusOK, `{"success":true,"needauth":false,"message":""}`),
	}}
	service.confirmationClient = confirmationapi.NewClient(confirmationapi.Options{
		Protocol: protocol.NewClient(protocol.Options{Transport: transport}), Offline: func() bool { return false },
	})
	token := issueConfirmationCapability(t, service)

	result, err := service.ListConfirmations(token)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "fresh" || result.AccountLabel != "qr_account" || len(result.Rows) != 1 {
		t.Fatalf("list result = %#v", result)
	}
	row := result.Rows[0]
	// A list refresh renders from getlist only, so no details are fetched.
	if row.Handle == "" || row.Title != "Trade offer" || row.Summary != "One & two" || len(row.Details) != 0 {
		t.Fatalf("row = %#v", row)
	}
	encoded := row.Title + row.Summary
	if strings.Contains(encoded, "100") || strings.Contains(encoded, "200") || strings.ContainsAny(encoded, "<>") {
		t.Fatalf("unsafe confirmation DTO = %#v", row)
	}

	decision, err := service.DecideConfirmation(row.Handle, "accept", token)
	if err != nil || decision.State != "ok" {
		t.Fatalf("decision = %#v, %v", decision, err)
	}
	if len(transport.requests) != 2 || !strings.Contains(transport.requests[1].URL.RawQuery, "op=allow") {
		t.Fatalf("decision request = %#v", transport.requests)
	}
	if _, err := service.DecideConfirmation(row.Handle, "deny", token); !errors.Is(err, capability.ErrInvalidCapability) {
		t.Fatalf("replayed decision error = %v", err)
	}
}

func TestConfirmationServiceRequiresWindowCapability(t *testing.T) {
	service, _ := setupQRService(t)
	issueConfirmationCapability(t, service)
	if _, err := service.ListConfirmations("wrong-capability"); !errors.Is(err, capability.ErrInvalidCapability) {
		t.Fatalf("capability error = %v", err)
	}
}

// liveConfirmationList is a real getlist payload: fields we do not consume plus
// the type_name/creation_time/icon a row is rendered from.
const liveConfirmationList = `{"success":true,"conf":[{"type":2,"type_name":"Trade Offer","id":"14412583061",` +
	`"creator_id":"7681695590","nonce":"10056326836841689505","creation_time":1753380000,"cancel":"Cancel",` +
	`"accept":"Confirm","icon":"https://avatars.akamai.steamstatic.com/abc_full.jpg","multi":false,` +
	`"headline":"tradepartner","summary":["You will give up 1 item"],"warn":null}]}`

func TestListConfirmationsRendersLiveRowsWithoutDetailsRequests(t *testing.T) {
	service, _ := setupQRService(t)
	transport := &confirmationQueueTransport{responses: []*http.Response{
		confirmationHTTPResponse(http.StatusOK, liveConfirmationList),
	}}
	service.confirmationClient = confirmationapi.NewClient(confirmationapi.Options{
		Protocol: protocol.NewClient(protocol.Options{Transport: transport}), Offline: func() bool { return false },
	})
	token := issueConfirmationCapability(t, service)

	result, err := service.ListConfirmations(token)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "fresh" || len(result.Rows) != 1 {
		t.Fatalf("list result = %#v", result)
	}
	row := result.Rows[0]
	// Kind, not TypeLabel, is what the window lays out from: the label is Steam's
	// own display string and it is free to reword it.
	if row.Kind != "trade" || row.TypeLabel != "Trade Offer" || row.Title != "tradepartner" ||
		row.Summary != "You will give up 1 item" || row.CreationTime != 1753380000 {
		t.Fatalf("row = %#v", row)
	}
	// The webview cannot load a Steam URL, so a row carries a locally served path
	// or nothing at all. Nothing is served here: this test has no network, which is
	// also the shape of a fetch Steam refuses.
	if row.Icon != "" && !strings.HasPrefix(row.Icon, "/img/confirmations/") {
		t.Fatalf("icon = %q, want a locally served path", row.Icon)
	}
	// Only getlist is requested; a details failure can no longer abort a refresh.
	if len(transport.requests) != 1 || !strings.Contains(transport.requests[0].URL.Path, "/mobileconf/getlist") {
		t.Fatalf("requests = %#v", transport.requests)
	}
}

func TestConfirmationCredentialsMintSessionIDWhenMissing(t *testing.T) {
	account := mafile.Account{
		DeviceID:       "android:01234567-89ab-cdef-0123-456789abcdef",
		IdentitySecret: "AAECAwQFBgcICQoLDA0ODxAREhM=",
		Session:        &mafile.SessionData{SteamID: 76561198000000000, AccessToken: "token-value"},
	}
	first, err := confirmationCredentials(account, "76561198000000000")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.SessionID) != 32 || strings.ToUpper(first.SessionID) != first.SessionID {
		t.Fatalf("session id = %q", first.SessionID)
	}
	if _, hexErr := strconv.ParseUint(first.SessionID[:16], 16, 64); hexErr != nil {
		t.Fatalf("session id is not hex: %q", first.SessionID)
	}
	second, err := confirmationCredentials(account, "76561198000000000")
	if err != nil {
		t.Fatal(err)
	}
	if second.SessionID == first.SessionID {
		t.Fatal("minted session id was reused")
	}
	// The generated value stays in memory; the stored account is untouched.
	if account.Session.SessionID != "" {
		t.Fatalf("session id was persisted: %q", account.Session.SessionID)
	}
}

func TestConfirmationSignaturesUseSteamAlignedTime(t *testing.T) {
	state := otp.NewTimeState(nil)
	if err := state.AcceptSample(time.Now().Add(90*time.Second).Unix(), time.Now()); err != nil {
		t.Fatal(err)
	}
	clock := steamAlignedClock{state: state}
	if drift := clock.Now().Sub(time.Now()); drift < 80*time.Second || drift > 100*time.Second {
		t.Fatalf("aligned clock drift = %s", drift)
	}
	if (steamAlignedClock{}).Now().IsZero() {
		t.Fatal("aligned clock without state must fall back to the system clock")
	}
}

func issueConfirmationCapability(t *testing.T, service *Service) string {
	t.Helper()
	binding := capability.Binding{
		WindowName: confirmationsWindowName, AccountID: qrTestAccountID,
		Scope: confirmationsCapabilityScope, LeaseID: "confirmation-window-instance",
		VaultGeneration: service.vault.Generation(),
	}
	service.confirmationWindowMu.Lock()
	service.confirmationAccountID = binding.AccountID
	service.confirmationGeneration = binding.VaultGeneration
	service.confirmationInstanceID = binding.LeaseID
	service.confirmationWindowMu.Unlock()
	token, err := service.capabilities.Issue(binding)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

// Opening a trade and the ten-second list poll must not share one cancel slot:
// whichever started second kills the first, leaving the trade blank or its icon
// tiles half-filled.
func TestConfirmationDetailAndListRefreshDoNotCancelEachOther(t *testing.T) {
	service := &Service{}

	listCtx, listOperation := service.beginConfirmationOperation()
	detailCtx, detailOperation := service.beginConfirmationDetail()
	if listCtx.Err() != nil {
		t.Fatal("opening a confirmation cancelled the list refresh")
	}

	// The poll that lands while the trade is still loading.
	nextListCtx, nextListOperation := service.beginConfirmationOperation()
	if detailCtx.Err() != nil {
		t.Fatal("a list refresh cancelled the open confirmation's detail fetch")
	}
	if listCtx.Err() == nil {
		t.Fatal("the superseded list refresh should still have been cancelled")
	}

	// Opening another row does still replace the one before it.
	if _, _ = service.beginConfirmationDetail(); detailCtx.Err() == nil {
		t.Fatal("opening another confirmation left the previous detail fetch running")
	}
	if nextListCtx.Err() != nil {
		t.Fatal("opening another confirmation cancelled the list refresh")
	}

	// Tearing the window down takes both.
	service.resetConfirmationSession(false)
	if nextListCtx.Err() == nil {
		t.Fatal("closing the window left a list refresh running")
	}
	service.endConfirmationOperation(listOperation)
	service.endConfirmationOperation(nextListOperation)
	service.endConfirmationDetail(detailOperation)
}

// A refresh prunes every icon the list does not name. A detail or hover fetch in
// flight is downloading icons it cannot name yet, and pruning deleted them as
// they landed — leaving the window valid paths to files that no longer existed.
func TestConfirmationIconsAreNotPrunedWhileAFetchIsDownloadingThem(t *testing.T) {
	service := &Service{}
	if service.confirmationIconsBusy() {
		t.Fatal("nothing is fetching, so a refresh should prune")
	}

	releaseDetail := service.holdConfirmationIcons()
	if !service.confirmationIconsBusy() {
		t.Fatal("a detail fetch is downloading icons, so a refresh must not prune")
	}

	// A hover overlapping the detail: the last one out lifts the hold, not the
	// first, or the detail's own icons go while the hover is still running.
	releaseHover := service.holdConfirmationIcons()
	releaseDetail()
	if !service.confirmationIconsBusy() {
		t.Fatal("an overlapping fetch lifted the hold early")
	}
	releaseHover()
	if service.confirmationIconsBusy() {
		t.Fatal("the hold outlived the fetches it was protecting")
	}
}

type confirmationQueueTransport struct {
	responses []*http.Response
	requests  []*http.Request
}

func (t *confirmationQueueTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	t.requests = append(t.requests, request.Clone(request.Context()))
	if len(t.responses) == 0 {
		return nil, errors.New("unexpected confirmation request")
	}
	response := t.responses[0]
	t.responses = t.responses[1:]
	response.Request = request
	return response, nil
}

func confirmationHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)),
	}
}
