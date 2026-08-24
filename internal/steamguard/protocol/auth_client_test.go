package protocol

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testSteamID = uint64(76561198000000000)

const testMobileAccessToken = "mobile.token+slash/value=1&next?"

func TestBeginAuthSessionViaCredentials(t *testing.T) {
	t.Parallel()

	var transportCalls atomic.Int32
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		transportCalls.Add(1)
		assertAuthRequest(t, request, beginCredentialsEndpoint)
		message := decodeAuthForm(t, request)
		fields := decodeFields(t, message)
		if len(fields) != 11 {
			t.Fatalf("request fields = %d, want 11", len(fields))
		}
		if got := string(fields[1].bytes); got != "TcNo Account Switcher" {
			t.Fatalf("device name = %q", got)
		}
		if got := string(fields[2].bytes); got != "account_name" {
			t.Fatalf("account name = %q", got)
		}
		if fields[4].varint != 123456 || fields[6].varint != uint64(PlatformMobileApp) || fields[7].varint != uint64(PersistenceEphemeral) {
			t.Fatalf("begin scalar fields are incorrect: %#v", fields)
		}
		deviceFields := decodeFields(t, fields[9].bytes)
		if string(deviceFields[1].bytes) != "TcNo Account Switcher" || deviceFields[2].varint != uint64(PlatformMobileApp) || deviceFields[7].varint != uint64(AppTypeSteamMobile) {
			t.Fatalf("device details are incorrect: %#v", deviceFields)
		}

		challenge := appendVarintField(nil, 1, uint64(ChallengeDeviceCode))
		challenge = appendStringField(challenge, 2, "Steam Guard app")
		body := appendVarintField(nil, 1, 4123)
		body = appendBytesField(body, 2, []byte{1, 2, 3, 4})
		body = appendFloat32Field(body, 3, 5)
		body = appendBytesField(body, 4, challenge)
		body = appendVarintField(body, 5, testSteamID)
		return response(request, http.StatusOK, nil, body), nil
	})

	entropy := bytes.Repeat([]byte{0xa5}, localSessionIDBytes)
	auth := newAuthenticationClientForTest(NewClient(Options{Transport: transport}), bytes.NewReader(entropy))
	result, err := auth.BeginAuthSessionViaCredentials(context.Background(), validBeginRequest(), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != AuthResultChallengeRequired || result.Session.ClientID() != 4123 || result.Session.SteamID() != testSteamID || result.Session.PollInterval() != 5*time.Second {
		t.Fatalf("begin result = %#v", result)
	}
	if result.Session.ID() != base64.RawURLEncoding.EncodeToString(entropy) {
		t.Fatalf("local session ID = %q", result.Session.ID())
	}
	challenges := result.Session.Challenges()
	if len(challenges) != 1 || challenges[0].Type != ChallengeDeviceCode || challenges[0].AssociatedMessage != "Steam Guard app" {
		t.Fatalf("challenges = %#v", challenges)
	}
	challenges[0].Type = ChallengeNone
	if result.Session.Challenges()[0].Type != ChallengeDeviceCode {
		t.Fatal("Challenges returned mutable session storage")
	}
	sessionCopy := result.Session
	result.Session.Destroy()
	if validateAuthSession(sessionCopy) {
		t.Fatal("destroyed session copy remained valid")
	}
	if transportCalls.Load() != 1 {
		t.Fatalf("transport calls = %d", transportCalls.Load())
	}
}

// Live shape: Steam refuses a wrong account name or password with HTTP 200, a
// non-OK EResult and an empty body, which the response parser can only read as a
// malformed response. Without the header the body is all there is, so the parser
// has to name the check that refused it.
func TestBeginAuthSessionViaCredentialsReportsRefusal(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name    string
		header  http.Header
		code    Code
		eresult int
		detail  string
	}{
		{name: "refused", header: steamEResultHeader("5"), code: CodeSteamResult, eresult: 5},
		{name: "throttled", header: steamEResultHeader("84"), code: CodeSteamResult, eresult: 84},
		{name: "no header", code: CodeInvalidResponse, detail: "begin_missing_client_id"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
				return response(request, http.StatusOK, testCase.header, nil), nil
			})
			entropy := bytes.Repeat([]byte{0xa5}, localSessionIDBytes)
			auth := newAuthenticationClientForTest(NewClient(Options{Transport: transport}), bytes.NewReader(entropy))
			_, err := auth.BeginAuthSessionViaCredentials(context.Background(), validBeginRequest(), time.Second)
			protocolErr := assertProtocolCode(t, err, testCase.code)
			if testCase.eresult != 0 && (!protocolErr.HasEResult || protocolErr.EResult != testCase.eresult) {
				t.Fatalf("Steam result metadata = %#v", protocolErr)
			}
			if protocolErr.Detail != testCase.detail {
				t.Fatalf("detail = %q, want %q", protocolErr.Detail, testCase.detail)
			}
		})
	}
}

func TestUpdateAuthSessionWithSteamGuardCode(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		assertAuthRequest(t, request, updateGuardCodeEndpoint)
		message := decodeAuthForm(t, request)
		fields := decodeFields(t, message)
		if fields[1].varint != 4123 || fields[2].fixed64 != testSteamID || string(fields[3].bytes) != "ABCDE" || fields[4].varint != uint64(ChallengeDeviceCode) {
			t.Fatalf("guard request fields = %#v", fields)
		}
		body := appendStringField(nil, 7, "https://store.steampowered.com/account/")
		return response(request, http.StatusOK, nil, body), nil
	})
	auth := NewAuthenticationClient(NewClient(Options{Transport: transport}))
	result, err := auth.UpdateAuthSessionWithSteamGuardCode(context.Background(), SteamGuardCodeRequest{
		Session: testSession(),
		Code:    "ABCDE",
		Type:    ChallengeDeviceCode,
	}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != AuthResultAgreementRequired || result.AgreementURL != "https://store.steampowered.com/account/" {
		t.Fatalf("challenge result = %#v", result)
	}
}

// Steam adds fields to this response without notice, and rejecting them turned an
// accepted code into invalid_response. An empty body is the ordinary success shape.
func TestUpdateAuthSessionWithSteamGuardCodeToleratesUnknownAndEmptyBodies(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name string
		body []byte
	}{
		{name: "empty", body: nil},
		{name: "unknown field", body: appendVarintField(nil, 12, 1)},
		{
			name: "unknown field alongside the agreement URL",
			body: appendVarintField(appendStringField(nil, 7, "https://store.steampowered.com/account/"), 12, 1),
		},
		// Steam sends the agreement field even with no agreement pending. Neither
		// form says anything about the code, so neither may fail the submission.
		{name: "explicitly empty agreement URL", body: appendStringField(nil, 7, "")},
		{name: "agreement URL this client would not open", body: appendStringField(nil, 7, "https://evil.example.com/")},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
				return response(request, http.StatusOK, steamEResultHeader("1"), testCase.body), nil
			})
			auth := NewAuthenticationClient(NewClient(Options{Transport: transport}))
			result, err := auth.UpdateAuthSessionWithSteamGuardCode(context.Background(), SteamGuardCodeRequest{
				Session: testSession(),
				Code:    "ABCDE",
				Type:    ChallengeDeviceCode,
			}, time.Second)
			if err != nil {
				t.Fatalf("challenge error = %#v", err)
			}
			if testCase.name != "unknown field alongside the agreement URL" {
				if result.State != AuthResultChallengeAccepted || result.AgreementURL != "" {
					t.Fatalf("challenge result = %#v", result)
				}
			}
		})
	}
}

// invalid_response names which check rejected the body, so the cause is readable
// from a log line instead of needing another reproduction.
func TestUpdateAuthSessionWithSteamGuardCodeNamesTheRejectingCheck(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name   string
		body   []byte
		detail string
	}{
		{name: "not protobuf", body: []byte{0x08}, detail: "guard_code_trailing_bytes"},
		{
			name:   "repeated agreement field",
			body:   appendStringField(appendStringField(nil, 7, "https://store.steampowered.com/a"), 7, "https://store.steampowered.com/b"),
			detail: "guard_code_duplicate_agreement_field",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
				return response(request, http.StatusOK, steamEResultHeader("1"), testCase.body), nil
			})
			auth := NewAuthenticationClient(NewClient(Options{Transport: transport}))
			_, err := auth.UpdateAuthSessionWithSteamGuardCode(context.Background(), SteamGuardCodeRequest{
				Session: testSession(),
				Code:    "ABCDE",
				Type:    ChallengeDeviceCode,
			}, time.Second)
			var protocolErr *Error
			if !errors.As(err, &protocolErr) || protocolErr.Code != CodeInvalidResponse {
				t.Fatalf("challenge error = %#v", err)
			}
			if protocolErr.Detail != testCase.detail {
				t.Fatalf("detail = %q, want %q", protocolErr.Detail, testCase.detail)
			}
		})
	}
}

// A refused code arrives as HTTP 200 with a non-OK EResult, which would otherwise
// read as an accepted challenge.
func TestUpdateAuthSessionWithSteamGuardCodeReportsRefusal(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return response(request, http.StatusOK, steamEResultHeader("88"), nil), nil
	})
	auth := NewAuthenticationClient(NewClient(Options{Transport: transport}))
	_, err := auth.UpdateAuthSessionWithSteamGuardCode(context.Background(), SteamGuardCodeRequest{
		Session: testSession(),
		Code:    "ABCDE",
		Type:    ChallengeDeviceCode,
	}, time.Second)
	var protocolErr *Error
	if !errors.As(err, &protocolErr) || protocolErr.Code != CodeSteamResult || protocolErr.EResult != 88 {
		t.Fatalf("challenge error = %#v", err)
	}
}

func TestPollAuthSessionStatus(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		assertAuthRequest(t, request, pollAuthSessionEndpoint)
		fields := decodeFields(t, decodeAuthForm(t, request))
		if fields[1].varint != 4123 || !bytes.Equal(fields[2].bytes, []byte{1, 2, 3, 4}) {
			t.Fatalf("poll request fields = %#v", fields)
		}
		body := appendVarintField(nil, 1, 9999)
		body = appendStringField(body, 2, "https://s.team/q/1/9999")
		body = appendStringField(body, 3, "refresh.token")
		body = appendStringField(body, 4, "access.token")
		body = appendVarintField(body, 5, 1)
		body = appendStringField(body, 6, "account_name")
		body = appendStringField(body, 7, "guard-data")
		return response(request, http.StatusOK, nil, body), nil
	})
	auth := NewAuthenticationClient(NewClient(Options{Transport: transport}))
	result, err := auth.PollAuthSessionStatus(context.Background(), testSession(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != AuthResultAuthorized || result.Session.ClientID() != 9999 || result.RefreshToken != "refresh.token" || result.AccessToken != "access.token" || !result.HadRemoteInteraction {
		t.Fatalf("poll result = %#v", result)
	}
	if result.AccountName != "account_name" || result.GuardData != "guard-data" || result.ChallengeURL != "https://s.team/q/1/9999" {
		t.Fatalf("poll metadata = %#v", result)
	}
}

// Live shape: a session Steam has dropped answers with an empty body, the same
// bytes a session still waiting for approval sends. Only the EResult separates
// them, and reading a dropped session as waiting leaves the caller polling a
// session that can never authorize.
func TestPollAuthSessionStatusSeparatesWaitingFromDroppedSession(t *testing.T) {
	t.Parallel()

	waiting := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return response(request, http.StatusOK, steamEResultHeader("1"), appendVarintField(nil, 5, 0)), nil
	})
	auth := NewAuthenticationClient(NewClient(Options{Transport: waiting}))
	result, err := auth.PollAuthSessionStatus(context.Background(), testSession(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != AuthResultWaiting || result.HadRemoteInteraction {
		t.Fatalf("waiting poll result = %#v", result)
	}

	dropped := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return response(request, http.StatusOK, steamEResultHeader("9"), nil), nil
	})
	auth = NewAuthenticationClient(NewClient(Options{Transport: dropped}))
	_, err = auth.PollAuthSessionStatus(context.Background(), testSession(), time.Second)
	protocolErr := assertProtocolCode(t, err, CodeSteamResult)
	if !protocolErr.HasEResult || protocolErr.EResult != 9 {
		t.Fatalf("dropped session error = %#v", protocolErr)
	}
}

func TestGenerateAccessTokenForApp(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		assertAuthRequest(t, request, generateAccessEndpoint)
		fields := decodeFields(t, decodeAuthForm(t, request))
		if string(fields[1].bytes) != "refresh.token" || fields[2].fixed64 != testSteamID || fields[3].varint != uint64(RenewalAllow) {
			t.Fatalf("token request fields = %#v", fields)
		}
		body := appendStringField(nil, 1, "new.access.token")
		body = appendStringField(body, 2, "new.refresh.token")
		return response(request, http.StatusOK, nil, body), nil
	})
	auth := NewAuthenticationClient(NewClient(Options{Transport: transport}))
	result, err := auth.GenerateAccessTokenForApp(context.Background(), GenerateAccessTokenRequest{
		SteamID:      testSteamID,
		RefreshToken: "refresh.token",
		Renewal:      RenewalAllow,
	}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != AuthResultTokenIssued || result.AccessToken != "new.access.token" || result.RefreshToken != "new.refresh.token" {
		t.Fatalf("token result = %#v", result)
	}
}

func TestUpdateAuthSessionWithMobileConfirmation(t *testing.T) {
	t.Parallel()

	signature := bytes.Repeat([]byte{0x5a}, 32)
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		assertAuthenticatedAuthRequest(t, request, updateMobileEndpoint, testMobileAccessToken)
		if request.URL.RawQuery != "access_token=mobile.token%2Bslash%2Fvalue%3D1%26next%3F" {
			t.Fatalf("raw query = %q", request.URL.RawQuery)
		}
		fields := decodeFields(t, decodeAuthForm(t, request))
		if fields[1].varint != 7 || fields[2].varint != 7788 || fields[3].fixed64 != testSteamID || !bytes.Equal(fields[4].bytes, signature) || fields[5].varint != 1 || fields[6].varint != uint64(PersistencePersistent) {
			t.Fatalf("mobile confirmation fields = %#v", fields)
		}
		return response(request, http.StatusOK, steamEResultHeader("1"), nil), nil
	})
	auth := NewAuthenticationClient(NewClient(Options{Transport: transport}))
	result, err := auth.UpdateAuthSessionWithMobileConfirmation(context.Background(), MobileConfirmationRequest{
		AccessToken: testMobileAccessToken,
		Version:     7,
		ClientID:    7788,
		SteamID:     testSteamID,
		Signature:   signature,
		Confirm:     true,
		Persistence: PersistencePersistent,
	}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != AuthResultChallengeAccepted {
		t.Fatalf("mobile confirmation result = %#v", result)
	}
}

func TestGetAuthSessionInfo(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		assertAuthenticatedAuthRequest(t, request, getAuthSessionInfoEndpoint, testMobileAccessToken)
		fields := decodeFields(t, decodeAuthForm(t, request))
		if len(fields) != 1 || fields[1].varint != 7788 {
			t.Fatalf("session-info request fields = %#v", fields)
		}
		body := appendStringField(nil, 1, "203.0.113.42")
		body = appendStringField(body, 2, "-33.9,18.4")
		body = appendStringField(body, 3, "Cape Town")
		body = appendStringField(body, 4, "Western Cape")
		body = appendStringField(body, 5, "ZA")
		body = appendVarintField(body, 6, uint64(PlatformSteamClient))
		body = appendStringField(body, 7, "Desktop PC")
		body = appendVarintField(body, 8, 7)
		body = appendVarintField(body, 9, uint64(SecurityHistoryUsedPreviously))
		body = appendVarintField(body, 10, 1)
		body = appendVarintField(body, 11, 1)
		body = appendVarintField(body, 12, uint64(PersistencePersistent))
		body = appendVarintField(body, 13, 3)
		body = appendVarintField(body, 14, uint64(AppTypeSteamMobile))
		return response(request, http.StatusOK, steamEResultHeader("1"), body), nil
	})
	auth := NewAuthenticationClient(NewClient(Options{Transport: transport}))
	result, err := auth.GetAuthSessionInfo(context.Background(), AuthSessionInfoRequest{
		AccessToken: testMobileAccessToken,
		ClientID:    7788,
	}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result.IPAddress != "203.0.113.42" || result.GeoLocation != "-33.9,18.4" || result.City != "Cape Town" || result.State != "Western Cape" || result.Country != "ZA" {
		t.Fatalf("session-info location = %#v", result)
	}
	if result.Platform != PlatformSteamClient || result.DeviceFriendlyName != "Desktop PC" || result.Version != 7 || result.LoginHistory != SecurityHistoryUsedPreviously || !result.RequestorLocationMismatch || !result.HighUsageLogin || result.RequestedPersistence != PersistencePersistent || result.DeviceTrust != 3 || result.App != AppTypeSteamMobile {
		t.Fatalf("session-info security fields = %#v", result)
	}
}

func TestMobileConfirmationRequiresSuccessfulEResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header http.Header
		code   Code
		result int
	}{
		{name: "missing", code: CodeInvalidResponse},
		{name: "invalid", header: steamEResultHeader("01"), code: CodeInvalidResponse},
		{name: "access denied", header: steamEResultHeader("15"), code: CodeSteamResult, result: 15},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
				return response(request, http.StatusOK, test.header, nil), nil
			})
			auth := NewAuthenticationClient(NewClient(Options{Transport: transport}))
			_, err := auth.UpdateAuthSessionWithMobileConfirmation(context.Background(), validMobileConfirmationRequest(), time.Second)
			protocolErr := assertProtocolCode(t, err, test.code)
			if test.result != 0 && (!protocolErr.HasEResult || protocolErr.EResult != test.result) {
				t.Fatalf("Steam result metadata = %#v", protocolErr)
			}
			if strings.Contains(err.Error(), testMobileAccessToken) || strings.Contains(err.Error(), "access_token") {
				t.Fatalf("error leaked access token: %q", err)
			}
		})
	}
}

func TestAuthenticatedOperationsCancellationIsSecretFree(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		close(started)
		<-request.Context().Done()
		return nil, errors.New("transport included " + request.URL.String())
	})
	auth := NewAuthenticationClient(NewClient(Options{Transport: transport}))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := auth.GetAuthSessionInfo(ctx, AuthSessionInfoRequest{AccessToken: testMobileAccessToken, ClientID: 7788}, time.Second)
		done <- err
	}()
	<-started
	cancel()
	err := <-done
	assertProtocolCode(t, err, CodeCanceled)
	if !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), testMobileAccessToken) || strings.Contains(err.Error(), "access_token") {
		t.Fatalf("cancellation error = %q", err)
	}
}

func TestAuthenticatedOperationsEnforceBoundsBeforeTransport(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("must not run")
	})
	auth := NewAuthenticationClient(NewClient(Options{Transport: transport}))
	oversizedToken := strings.Repeat("x", maxTokenBytes+1)
	_, err := auth.GetAuthSessionInfo(context.Background(), AuthSessionInfoRequest{AccessToken: oversizedToken, ClientID: 7788}, time.Second)
	assertProtocolCode(t, err, CodeInvalidRequest)
	request := validMobileConfirmationRequest()
	request.Version = 1 << 16
	_, err = auth.UpdateAuthSessionWithMobileConfirmation(context.Background(), request, time.Second)
	assertProtocolCode(t, err, CodeInvalidRequest)
	if calls.Load() != 0 {
		t.Fatalf("transport calls = %d", calls.Load())
	}
}

func TestGetAuthSessionInfoRejectsOversizedField(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := appendStringField(nil, 1, strings.Repeat("1", maxIPAddressBytes+1))
		body = appendVarintField(body, 6, uint64(PlatformSteamClient))
		body = appendStringField(body, 7, "Desktop PC")
		body = appendVarintField(body, 8, 1)
		return response(request, http.StatusOK, steamEResultHeader("1"), body), nil
	})
	auth := NewAuthenticationClient(NewClient(Options{Transport: transport}))
	_, err := auth.GetAuthSessionInfo(context.Background(), AuthSessionInfoRequest{AccessToken: testMobileAccessToken, ClientID: 7788}, time.Second)
	assertProtocolCode(t, err, CodeInvalidResponse)
}

func TestAuthenticationRejectsMalformedOrSecretBearingResponse(t *testing.T) {
	t.Parallel()

	const secret = "response-secret"
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := appendStringField(nil, 1, secret)
		body = appendStringField(body, 1, "duplicate")
		return response(request, http.StatusOK, nil, body), nil
	})
	auth := NewAuthenticationClient(NewClient(Options{Transport: transport}))
	_, err := auth.GenerateAccessTokenForApp(context.Background(), GenerateAccessTokenRequest{
		SteamID:      testSteamID,
		RefreshToken: "refresh.token",
	}, time.Second)
	assertProtocolCode(t, err, CodeInvalidResponse)
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("parse error leaked response token: %q", err)
	}
}

// Live shape: Steam adds fields to this response without notice, and the extra
// ones must not turn a usable token grant into an invalid response.
func TestGenerateAccessTokenForAppToleratesUnknownFields(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := appendStringField(nil, 1, "new.access.token")
		body = appendStringField(body, 2, "new.refresh.token")
		body = appendVarintField(body, 3, 1)
		body = appendStringField(body, 4, "agreement.session.url")
		body = appendVarintField(body, 70, 12345)
		body = appendVarintField(body, 70, 12346)
		return response(request, http.StatusOK, steamEResultHeader("1"), body), nil
	})
	auth := NewAuthenticationClient(NewClient(Options{Transport: transport}))
	result, err := auth.GenerateAccessTokenForApp(context.Background(), GenerateAccessTokenRequest{
		SteamID:      testSteamID,
		RefreshToken: "refresh.token",
		Renewal:      RenewalAllow,
	}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result.AccessToken != "new.access.token" || result.RefreshToken != "new.refresh.token" {
		t.Fatalf("token result = %#v", result)
	}
}

// A refused refresh token comes back as HTTP 200, a non-OK x-eresult and an
// empty body. Callers route a rejection to the sign-in form and an invalid
// response to an error, so the two must stay distinguishable.
func TestGenerateAccessTokenForAppReportsSteamRejection(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return response(request, http.StatusOK, steamEResultHeader("16"), nil), nil
	})
	auth := NewAuthenticationClient(NewClient(Options{Transport: transport}))
	_, err := auth.GenerateAccessTokenForApp(context.Background(), GenerateAccessTokenRequest{
		SteamID:      testSteamID,
		RefreshToken: "refresh.token",
	}, time.Second)
	assertProtocolCode(t, err, CodeSteamResult)
	var protocolErr *Error
	if !errors.As(err, &protocolErr) || !protocolErr.HasEResult || protocolErr.EResult != 16 {
		t.Fatalf("rejection error = %#v", err)
	}
}

func TestAuthenticationAppliesOperationResponseLimit(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := bytes.Repeat([]byte{0x01}, maxAuthResponseBytes+1)
		result := response(request, http.StatusOK, nil, body)
		result.ContentLength = -1
		return result, nil
	})
	auth := NewAuthenticationClient(NewClient(Options{Transport: transport}))
	_, err := auth.GenerateAccessTokenForApp(context.Background(), GenerateAccessTokenRequest{
		SteamID:      testSteamID,
		RefreshToken: "refresh.token",
	}, time.Second)
	assertProtocolCode(t, err, CodeResponseTooLarge)
}

func TestAuthenticationCancellationPreservesTypedError(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		close(started)
		<-request.Context().Done()
		return nil, request.Context().Err()
	})
	auth := NewAuthenticationClient(NewClient(Options{Transport: transport}))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := auth.GenerateAccessTokenForApp(ctx, GenerateAccessTokenRequest{
			SteamID:      testSteamID,
			RefreshToken: "refresh.token",
		}, time.Second)
		done <- err
	}()
	<-started
	cancel()
	err := <-done
	assertProtocolCode(t, err, CodeCanceled)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("errors.Is(context.Canceled) = false: %v", err)
	}
}

func TestAuthenticationRejectsInvalidInputBeforeTransport(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("must not run")
	})
	auth := NewAuthenticationClient(NewClient(Options{Transport: transport}))
	request := validBeginRequest()
	request.EncryptedPassword = "plaintext-password"
	_, err := auth.BeginAuthSessionViaCredentials(context.Background(), request, time.Second)
	assertProtocolCode(t, err, CodeInvalidRequest)
	if calls.Load() != 0 {
		t.Fatalf("transport calls = %d", calls.Load())
	}
}

func TestBeginAuthSessionReportsEntropyFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		challenge := appendVarintField(nil, 1, uint64(ChallengeDeviceCode))
		body := appendVarintField(nil, 1, 4123)
		body = appendBytesField(body, 2, []byte{1, 2, 3, 4})
		body = appendFloat32Field(body, 3, 5)
		body = appendBytesField(body, 4, challenge)
		body = appendVarintField(body, 5, testSteamID)
		return response(request, http.StatusOK, nil, body), nil
	})
	auth := newAuthenticationClientForTest(NewClient(Options{Transport: transport}), errReader{})
	_, err := auth.BeginAuthSessionViaCredentials(context.Background(), validBeginRequest(), time.Second)
	assertProtocolCode(t, err, CodeEntropy)
	if calls.Load() != 0 {
		t.Fatalf("transport calls = %d", calls.Load())
	}
}

func TestEncodeProtobufFormEscapesStandardBase64(t *testing.T) {
	t.Parallel()

	got := string(encodeProtobufForm([]byte{0xfb, 0xff}))
	if got != "input_protobuf_encoded=%2B%2F8%3D" {
		t.Fatalf("encoded form = %q", got)
	}
}

func validBeginRequest() BeginCredentialsRequest {
	encrypted := base64.StdEncoding.EncodeToString(make([]byte, 256))
	return BeginCredentialsRequest{
		DeviceFriendlyName:  "TcNo Account Switcher",
		AccountName:         "account_name",
		EncryptedPassword:   encrypted,
		EncryptionTimestamp: 123456,
		RememberLogin:       false,
		Platform:            PlatformMobileApp,
		Persistence:         PersistenceEphemeral,
		WebsiteID:           "Mobile",
		Device: DeviceDetails{
			FriendlyName: "TcNo Account Switcher",
			Platform:     PlatformMobileApp,
			OSType:       32,
			MachineID:    []byte("machine-id"),
			App:          AppTypeSteamMobile,
		},
		Language: 0,
		QoSLevel: 2,
	}
}

func testSession() AuthSession {
	return AuthSession{
		id:           base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x11}, localSessionIDBytes)),
		clientID:     4123,
		requestID:    []byte{1, 2, 3, 4},
		steamID:      testSteamID,
		pollInterval: 5 * time.Second,
	}
}

func validMobileConfirmationRequest() MobileConfirmationRequest {
	return MobileConfirmationRequest{
		AccessToken: testMobileAccessToken,
		Version:     1,
		ClientID:    7788,
		SteamID:     testSteamID,
		Signature:   bytes.Repeat([]byte{0x5a}, 32),
		Confirm:     true,
		Persistence: PersistencePersistent,
	}
}

func assertAuthRequest(t *testing.T, request *http.Request, endpoint string) {
	t.Helper()
	if request.URL.String() != endpoint {
		t.Fatalf("endpoint = %q, want %q", request.URL, endpoint)
	}
	if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || request.Header.Get("Accept") != "application/x-protobuf" {
		t.Fatalf("request metadata = %s %#v", request.Method, request.Header)
	}
	deadline, ok := request.Context().Deadline()
	if !ok || time.Until(deadline) <= 0 {
		t.Fatal("authentication request has no active deadline")
	}
}

// assertReadOnlyAuthRequest pins the GET shape Steam requires for its read-only
// service methods: payload in the query, no body, and no Content-Type.
func assertReadOnlyAuthRequest(t *testing.T, request *http.Request, endpoint string) {
	t.Helper()
	if request.URL.Scheme+"://"+request.URL.Host+request.URL.Path != endpoint {
		t.Fatalf("endpoint = %q, want base %q", request.URL, endpoint)
	}
	if request.Method != http.MethodGet || request.Header.Get("Accept") != "application/x-protobuf" {
		t.Fatalf("request metadata = %s %#v", request.Method, request.Header)
	}
	if request.Header.Get("Content-Type") != "" {
		t.Fatalf("read-only request sent Content-Type %q", request.Header.Get("Content-Type"))
	}
	if request.Body != nil && request.ContentLength > 0 {
		t.Fatalf("read-only request sent a %d byte body", request.ContentLength)
	}
	deadline, ok := request.Context().Deadline()
	if !ok || time.Until(deadline) <= 0 {
		t.Fatal("authentication request has no active deadline")
	}
}

// decodeAuthQuery reads the protobuf payload a read-only method carries in its query.
func decodeAuthQuery(t *testing.T, request *http.Request) []byte {
	t.Helper()
	values := request.URL.Query()
	if len(values) != 1 || len(values["input_protobuf_encoded"]) != 1 {
		t.Fatalf("query fields = %#v", values)
	}
	message, err := base64.StdEncoding.DecodeString(values.Get("input_protobuf_encoded"))
	if err != nil {
		t.Fatal(err)
	}
	return message
}

func assertAuthenticatedAuthRequest(t *testing.T, request *http.Request, endpoint, accessToken string) {
	t.Helper()
	parsed, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if request.URL.Scheme != parsed.Scheme || request.URL.Host != parsed.Host || request.URL.Path != parsed.Path {
		t.Fatalf("endpoint = %q, want base %q", request.URL, endpoint)
	}
	if request.URL.Query().Get("access_token") != accessToken || len(request.URL.Query()) != 1 {
		t.Fatalf("authentication query is incorrect")
	}
	if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || request.Header.Get("Accept") != "application/x-protobuf" {
		t.Fatalf("request metadata = %s %#v", request.Method, request.Header)
	}
}

func steamEResultHeader(value string) http.Header {
	return http.Header{"X-EResult": {value}}
}

func decodeAuthForm(t *testing.T, request *http.Request) []byte {
	t.Helper()
	body, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatal(err)
	}
	values, err := url.ParseQuery(string(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || len(values["input_protobuf_encoded"]) != 1 {
		t.Fatalf("form fields = %#v", values)
	}
	message, err := base64.StdEncoding.DecodeString(values.Get("input_protobuf_encoded"))
	if err != nil {
		t.Fatal(err)
	}
	return message
}

func decodeFields(t *testing.T, data []byte) map[uint32]protobufField {
	t.Helper()
	fields := make(map[uint32]protobufField)
	decoder := protobufDecoder{data: data}
	for {
		field, ok := decoder.next()
		if !ok {
			break
		}
		if _, exists := fields[field.number]; exists {
			t.Fatalf("duplicate field %d", field.number)
		}
		fields[field.number] = field
	}
	if !decoder.validEnd() {
		t.Fatal("invalid protobuf message")
	}
	return fields
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("entropy secret") }
