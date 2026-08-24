package protocol

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

const testQRClientID = uint64(9999)

func validQRRequest() BeginQRRequest {
	return BeginQRRequest{
		DeviceFriendlyName: "TcNo Account Switcher",
		Platform:           PlatformMobileApp,
		Device: DeviceDetails{
			FriendlyName: "TcNo Account Switcher",
			Platform:     PlatformMobileApp,
			OSType:       32,
			MachineID:    []byte("machine-id"),
			App:          AppTypeSteamMobile,
		},
		WebsiteID: "Mobile",
	}
}

func qrResponseBody(clientID uint64, challengeURL string) []byte {
	challenge := appendVarintField(nil, 1, uint64(ChallengeDeviceConfirmation))
	body := appendVarintField(nil, 1, clientID)
	body = appendStringField(body, 2, challengeURL)
	body = appendBytesField(body, 3, []byte{1, 2, 3, 4})
	body = appendFloat32Field(body, 4, 5)
	body = appendBytesField(body, 5, challenge)
	return appendVarintField(body, 6, 2)
}

func TestBeginAuthSessionViaQR(t *testing.T) {
	t.Parallel()

	var transportCalls atomic.Int32
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		transportCalls.Add(1)
		assertAuthRequest(t, request, beginQREndpoint)
		fields := decodeFields(t, decodeAuthForm(t, request))
		if len(fields) != 4 {
			t.Fatalf("request fields = %d, want 4", len(fields))
		}
		if got := string(fields[1].bytes); got != "TcNo Account Switcher" {
			t.Fatalf("device name = %q", got)
		}
		if fields[2].varint != uint64(PlatformMobileApp) {
			t.Fatalf("platform = %d", fields[2].varint)
		}
		if got := string(fields[4].bytes); got != "Mobile" {
			t.Fatalf("website id = %q", got)
		}
		deviceFields := decodeFields(t, fields[3].bytes)
		if string(deviceFields[1].bytes) != "TcNo Account Switcher" ||
			deviceFields[2].varint != uint64(PlatformMobileApp) ||
			deviceFields[7].varint != uint64(AppTypeSteamMobile) {
			t.Fatalf("device details are incorrect: %#v", deviceFields)
		}
		body := qrResponseBody(testQRClientID, fmt.Sprintf("https://s.team/q/1/%d", testQRClientID))
		return response(request, http.StatusOK, nil, body), nil
	})

	entropy := bytes.Repeat([]byte{0xa5}, localSessionIDBytes)
	auth := newAuthenticationClientForTest(NewClient(Options{Transport: transport}), bytes.NewReader(entropy))
	result, err := auth.BeginAuthSessionViaQR(context.Background(), validQRRequest(), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != AuthResultWaiting || result.Session.ClientID() != testQRClientID || result.Version != 2 {
		t.Fatalf("qr begin result = %#v", result)
	}
	if result.ChallengeURL != fmt.Sprintf("https://s.team/q/1/%d", testQRClientID) {
		t.Fatalf("challenge URL = %q", result.ChallengeURL)
	}
	if result.Session.PollInterval() != 5*time.Second {
		t.Fatalf("poll interval = %s", result.Session.PollInterval())
	}
	if result.Session.ID() != base64.RawURLEncoding.EncodeToString(entropy) {
		t.Fatalf("local session ID = %q", result.Session.ID())
	}
	// Nobody has scanned it yet, so there is no account to name.
	if result.Session.SteamID() != 0 || !result.Session.ViaQR() {
		t.Fatalf("qr session identity = %d, viaQR %t", result.Session.SteamID(), result.Session.ViaQR())
	}
	if transportCalls.Load() != 1 {
		t.Fatalf("transport calls = %d", transportCalls.Load())
	}
}

// The URL becomes an image the user is told to scan, so a response that names
// one session and hands back a sign-in for another must not get through.
func TestBeginAuthSessionViaQRRejectsMismatchedChallengeURL(t *testing.T) {
	t.Parallel()

	for name, challengeURL := range map[string]string{
		"other session": fmt.Sprintf("https://s.team/q/1/%d", testQRClientID+1),
		"other host":    "https://evil.example/q/1/9999",
		"not a url":     "q/1/9999",
		"empty":         "",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
				return response(request, http.StatusOK, nil, qrResponseBody(testQRClientID, challengeURL)), nil
			})
			entropy := bytes.Repeat([]byte{0xa5}, localSessionIDBytes)
			auth := newAuthenticationClientForTest(NewClient(Options{Transport: transport}), bytes.NewReader(entropy))
			_, err := auth.BeginAuthSessionViaQR(context.Background(), validQRRequest(), time.Second)
			if err == nil {
				t.Fatal("a challenge URL that does not name this session was accepted")
			}
		})
	}
}

func TestPollAcceptsQRSessionWithoutSteamID(t *testing.T) {
	t.Parallel()

	qrSession := AuthSession{
		id:        base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0xa5}, localSessionIDBytes)),
		clientID:  testQRClientID,
		requestID: []byte{1, 2, 3, 4},
		viaQR:     true,
	}
	if !validatePollableSession(qrSession) {
		t.Fatal("a QR session waiting to be scanned was refused by the poll")
	}
	// The same session without the marker is a credentials session missing its
	// account, which is what the SteamID check exists to refuse.
	credentials := qrSession
	credentials.viaQR = false
	if validatePollableSession(credentials) {
		t.Fatal("a credentials session with no SteamID was accepted")
	}
	// A guard code is submitted for a named account, so that path keeps the
	// stricter rule whatever the session is.
	if validateAuthSession(qrSession) {
		t.Fatal("a QR session was accepted where a named account is required")
	}
}
