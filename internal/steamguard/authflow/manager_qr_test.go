package authflow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/protocol"
)

const (
	testQRClientID    = uint64(4123)
	testQRAccountName = "account_name"
)

func testQRChallengeURL(clientID uint64) string {
	return fmt.Sprintf("https://s.team/q/1/%d", clientID)
}

// newProtocolQRSession builds a real QR session, because viaQR is unexported and
// BeginAuthSessionViaQR is the only thing that sets it - which is the point.
func newProtocolQRSession(t *testing.T, clientID uint64) protocol.BeginQRResult {
	t.Helper()
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		confirmation := appendProtoVarint(nil, 1, uint64(protocol.ChallengeDeviceConfirmation))
		body := appendProtoVarint(nil, 1, clientID)
		body = appendProtoBytes(body, 2, []byte(testQRChallengeURL(clientID)))
		body = appendProtoBytes(body, 3, []byte("request-id-secret"))
		body = appendProtoFixed32(body, 4, math.Float32bits(5))
		body = appendProtoBytes(body, 5, confirmation)
		return &http.Response{
			StatusCode:    http.StatusOK,
			Status:        "200 OK",
			Header:        make(http.Header),
			Body:          io.NopCloser(bytes.NewReader(body)),
			ContentLength: int64(len(body)),
			Request:       request,
		}, nil
	})
	auth := protocol.NewAuthenticationClient(protocol.NewClient(protocol.Options{Transport: transport}))
	result, err := auth.BeginAuthSessionViaQR(context.Background(), testQRRequest(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func testQRRequest() protocol.BeginQRRequest {
	return protocol.BeginQRRequest{
		DeviceFriendlyName: "TcNo Account Switcher",
		Platform:           protocol.PlatformMobileApp,
		Device: protocol.DeviceDetails{
			FriendlyName: "TcNo Account Switcher",
			Platform:     protocol.PlatformMobileApp,
			OSType:       32,
			MachineID:    []byte("machine-id"),
			App:          protocol.AppTypeSteamMobile,
		},
		WebsiteID: "Mobile",
	}
}

func testQRBinding(account string) Binding {
	binding := testBinding(account)
	binding.ExpectedAccountName = testQRAccountName
	return binding
}

func qrFakeClient(t *testing.T, clientID uint64) *fakeClient {
	t.Helper()
	client := defaultFakeClient(t, protocol.ChallengeDeviceConfirmation)
	client.beginQR = func(context.Context, protocol.BeginQRRequest, time.Duration) (protocol.BeginQRResult, error) {
		return newProtocolQRSession(t, clientID), nil
	}
	return client
}

func beginTestQRSession(t *testing.T, manager *Manager, binding Binding) Status {
	t.Helper()
	status, err := manager.BeginQR(context.Background(), binding, testQRRequest())
	if err != nil {
		t.Fatal(err)
	}
	return status
}

// The unlock screen shows the QR code and the password form together, so one
// must not be refused because the other is open.
func TestBeginQRRunsAlongsideThePasswordSession(t *testing.T) {
	clock := newFakeClock()
	client := qrFakeClient(t, testQRClientID)
	manager := newTestManager(t, client, clock, uniqueEntropy(6), 4)
	defer manager.Close()
	binding := testQRBinding("account-a")

	password := beginTestSession(t, manager, binding)
	qr := beginTestQRSession(t, manager, binding)
	if qr.Handle == password.Handle {
		t.Fatal("the QR sign-in reused the password sign-in's session")
	}
	if qr.State != StateWaiting || qr.ChallengeURL != testQRChallengeURL(testQRClientID) {
		t.Fatalf("qr status = %#v", qr)
	}
	if password.ChallengeURL != "" {
		t.Fatal("a password session reported a challenge URL")
	}
	// One QR sign-in per account is still the limit.
	if _, err := manager.BeginQR(context.Background(), binding, testQRRequest()); err == nil {
		t.Fatal("a second QR session for one account was allowed")
	} else {
		assertFlowKind(t, err, ErrorConflict)
	}
}

func TestBeginQRRequiresAnExpectedAccountName(t *testing.T) {
	clock := newFakeClock()
	client := qrFakeClient(t, testQRClientID)
	manager := newTestManager(t, client, clock, uniqueEntropy(4), 2)
	defer manager.Close()

	binding := testBinding("account-a")
	binding.ExpectedAccountName = ""
	_, err := manager.BeginQR(context.Background(), binding, testQRRequest())
	assertFlowKind(t, err, ErrorInvalid)
}

// Whoever scans decides which account signs in, so a scan by somebody else has
// to be refused before their tokens are handed to anything.
func TestPollRefusesQRAuthorisationForAnotherAccount(t *testing.T) {
	clock := newFakeClock()
	client := qrFakeClient(t, testQRClientID)
	client.poll = func(_ context.Context, session protocol.AuthSession, _ time.Duration) (protocol.PollResult, error) {
		return protocol.PollResult{
			State:        protocol.AuthResultAuthorized,
			Session:      session,
			AccessToken:  "access-token-secret",
			RefreshToken: "refresh-token-secret",
			AccountName:  "somebody_else",
		}, nil
	}
	manager := newTestManager(t, client, clock, uniqueEntropy(4), 2)
	defer manager.Close()
	binding := testQRBinding("account-a")
	status := beginTestQRSession(t, manager, binding)

	clock.Advance(5 * time.Second)
	_, err := manager.Poll(context.Background(), binding, status.Handle)
	assertFlowKind(t, err, ErrorBindingMismatch)

	consumeErr := manager.Consume(binding, status.Handle, func(uint64, []byte, []byte, []byte, []byte, bool) error {
		t.Fatal("credentials for another account reached the consumer")
		return nil
	})
	if consumeErr == nil {
		t.Fatal("a refused QR sign-in still had credentials to consume")
	}
}

func TestPollAcceptsQRAuthorisationForTheExpectedAccount(t *testing.T) {
	clock := newFakeClock()
	client := qrFakeClient(t, testQRClientID)
	client.poll = func(_ context.Context, session protocol.AuthSession, _ time.Duration) (protocol.PollResult, error) {
		return protocol.PollResult{
			State:        protocol.AuthResultAuthorized,
			Session:      session,
			AccessToken:  "access-token-secret",
			RefreshToken: "refresh-token-secret",
			// Steam echoes the name as the account spells it; the check is
			// case-insensitive because Steam's own comparison is.
			AccountName: "Account_Name",
		}, nil
	}
	manager := newTestManager(t, client, clock, uniqueEntropy(4), 2)
	defer manager.Close()
	binding := testQRBinding("account-a")
	status := beginTestQRSession(t, manager, binding)

	clock.Advance(5 * time.Second)
	status, err := manager.Poll(context.Background(), binding, status.Handle)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateAuthorizedReady {
		t.Fatalf("qr poll status = %#v", status)
	}
	var seenName string
	if consumeErr := manager.Consume(binding, status.Handle, func(_ uint64, accountName, _, refreshToken, _ []byte, _ bool) error {
		seenName = string(accountName)
		if len(refreshToken) == 0 {
			t.Fatal("no refresh token reached the consumer")
		}
		return nil
	}); consumeErr != nil {
		t.Fatal(consumeErr)
	}
	if seenName != "Account_Name" {
		t.Fatalf("consumed account name = %q", seenName)
	}
}

// Steam replaces the code while it waits to be scanned. That arrives as a
// challenge, but there is nothing for the user to answer - the screen just has a
// new image to draw - so the session has to stay waiting.
func TestPollRotatesTheQRCodeWithoutAskingForAChallenge(t *testing.T) {
	clock := newFakeClock()
	rotatedClientID := testQRClientID + 1
	client := qrFakeClient(t, testQRClientID)
	client.poll = func(_ context.Context, session protocol.AuthSession, _ time.Duration) (protocol.PollResult, error) {
		return protocol.PollResult{
			State:        protocol.AuthResultChallengeRequired,
			Session:      session,
			ChallengeURL: testQRChallengeURL(rotatedClientID),
		}, nil
	}
	manager := newTestManager(t, client, clock, uniqueEntropy(4), 2)
	defer manager.Close()
	binding := testQRBinding("account-a")
	status := beginTestQRSession(t, manager, binding)

	clock.Advance(5 * time.Second)
	status, err := manager.Poll(context.Background(), binding, status.Handle)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateWaiting || !status.CanPoll {
		t.Fatalf("rotated qr status = %#v", status)
	}
	if status.ChallengeURL != testQRChallengeURL(rotatedClientID) {
		t.Fatalf("challenge URL = %q, want the rotated one", status.ChallengeURL)
	}
	if len(status.Challenges) != 0 {
		t.Fatalf("a rotated QR code asked for challenges: %#v", status.Challenges)
	}
}
