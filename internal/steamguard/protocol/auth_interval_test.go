package protocol

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"testing"
	"time"
)

func TestBeginAuthSessionPollingInterval(t *testing.T) {
	t.Parallel()
	for _, flow := range []struct {
		name          string
		intervalField uint32
		detailPrefix  string
		body          func() []byte
		begin         func(*AuthenticationClient) (AuthSession, error)
	}{
		{
			name: "credentials", intervalField: 3, detailPrefix: "begin_",
			body: func() []byte {
				body := appendVarintField(nil, 1, 4123)
				body = appendBytesField(body, 2, []byte{1, 2, 3, 4})
				body = appendBytesField(body, 4, appendVarintField(nil, 1, uint64(ChallengeNone)))
				return appendVarintField(body, 5, testSteamID)
			},
			begin: func(auth *AuthenticationClient) (AuthSession, error) {
				result, err := auth.BeginAuthSessionViaCredentials(context.Background(), validBeginRequest(), time.Second)
				return result.Session, err
			},
		},
		{
			name: "QR", intervalField: 4, detailPrefix: "qr_begin_",
			body: func() []byte {
				body := appendVarintField(nil, 1, testQRClientID)
				body = appendStringField(body, 2, fmt.Sprintf("https://s.team/q/1/%d", testQRClientID))
				body = appendBytesField(body, 3, []byte{1, 2, 3, 4})
				body = appendBytesField(body, 5, appendVarintField(nil, 1, uint64(ChallengeDeviceConfirmation)))
				return appendVarintField(body, 6, 2)
			},
			begin: func(auth *AuthenticationClient) (AuthSession, error) {
				result, err := auth.BeginAuthSessionViaQR(context.Background(), validQRRequest(), time.Second)
				return result.Session, err
			},
		},
	} {
		t.Run(flow.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				name    string
				seconds float32
				omit    bool
				want    time.Duration
				invalid bool
			}{
				{name: "omitted", omit: true, want: 5 * time.Second},
				{name: "zero", seconds: 0, want: 5 * time.Second},
				{name: "short delay", seconds: 0.125, want: 250 * time.Millisecond},
				{name: "smallest positive float", seconds: math.SmallestNonzeroFloat32, want: 250 * time.Millisecond},
				{name: "minimum delay", seconds: 0.25, want: 250 * time.Millisecond},
				{name: "usual delay", seconds: 5, want: 5 * time.Second},
				{name: "old upper limit", seconds: 60, want: time.Minute},
				{name: "above old upper limit", seconds: 61, want: 61 * time.Second},
				{name: "long delay", seconds: 3600, want: time.Hour},
				{name: "negative", seconds: -1, invalid: true},
				{name: "NaN", seconds: float32(math.NaN()), invalid: true},
				{name: "positive infinity", seconds: float32(math.Inf(1)), invalid: true},
				{name: "negative infinity", seconds: float32(math.Inf(-1)), invalid: true},
				{name: "duration overflow", seconds: math.MaxFloat32, invalid: true},
				{name: "largest representable delay", seconds: 9223371776, want: 9223371776 * time.Second},
				{name: "duration boundary", seconds: math.Nextafter32(float32(float64(math.MaxInt64)/float64(time.Second)), float32(math.Inf(1))), invalid: true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
						body := flow.body()
						if !tc.omit {
							body = appendFloat32Field(body, flow.intervalField, tc.seconds)
						}
						return response(request, http.StatusOK, steamEResultHeader("1"), body), nil
					})
					auth := NewAuthenticationClient(NewClient(Options{Transport: transport}))
					session, err := flow.begin(auth)
					defer session.Destroy()
					if tc.invalid {
						protocolErr := assertProtocolCode(t, err, CodeInvalidResponse)
						if protocolErr.Detail != flow.detailPrefix+"interval_range" {
							t.Fatalf("error detail = %q", protocolErr.Detail)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					if session.PollInterval() != tc.want {
						t.Fatalf("poll interval = %s, want %s", session.PollInterval(), tc.want)
					}
				})
			}
		})
	}
}

func TestBeginAuthSessionPollingIntervalRejectsMalformedFields(t *testing.T) {
	t.Parallel()
	for _, flow := range []struct {
		name          string
		intervalField uint32
		prefix        string
		parse         func([]byte) *Error
	}{
		{"credentials", 3, "begin_", func(body []byte) *Error { _, err := unmarshalBeginCredentialsResponse(body); return err }},
		{"QR", 4, "qr_begin_", func(body []byte) *Error { _, err := unmarshalBeginQRResponse(body); return err }},
	} {
		t.Run(flow.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				name   string
				body   []byte
				detail string
			}{
				{"wrong wire type", appendVarintField(nil, flow.intervalField, 5), "interval_field"},
				{"duplicate", appendFloat32Field(appendFloat32Field(nil, flow.intervalField, 0), flow.intervalField, 5), "interval_field"},
				{"truncated float", []byte{byte(flow.intervalField<<3 | wireFixed32), 0}, "trailing_bytes"},
				{"missing client ID", nil, "missing_client_id"},
				{"missing request ID", appendVarintField(nil, 1, 4123), "missing_request_id"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					err := flow.parse(tc.body)
					if err == nil || err.Code != CodeInvalidResponse || err.Detail != flow.prefix+tc.detail {
						t.Fatalf("parse error = %v, want invalid_response: %s%s", err, flow.prefix, tc.detail)
					}
				})
			}
		})
	}
}

func TestCredentialLoginWithZeroIntervalStillRequiresPoll(t *testing.T) {
	t.Parallel()
	var polls int
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var body []byte
		switch "https://" + request.URL.Host + request.URL.Path {
		case beginCredentialsEndpoint:
			body = appendVarintField(nil, 1, 4123)
			body = appendBytesField(body, 2, []byte{1, 2, 3, 4})
			body = appendFloat32Field(body, 3, 0)
			body = appendBytesField(body, 4, appendVarintField(nil, 1, uint64(ChallengeNone)))
			body = appendVarintField(body, 5, testSteamID)
		case pollAuthSessionEndpoint:
			polls++
			if polls == 2 {
				body = appendStringField(nil, 3, "issued.refresh.token")
				body = appendStringField(body, 4, "issued.access.token")
				body = appendStringField(body, 6, "account_name")
			}
		default:
			t.Fatalf("unexpected endpoint: %s", request.URL.Path)
		}
		return response(request, http.StatusOK, steamEResultHeader("1"), body), nil
	})
	auth := NewAuthenticationClient(NewClient(Options{Transport: transport}))
	begin, err := auth.BeginAuthSessionViaCredentials(context.Background(), validBeginRequest(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer begin.Session.Destroy()
	if begin.State != AuthResultWaiting || begin.Session.PollInterval() != 5*time.Second {
		t.Fatal("begin did not return a waiting session with a safe poll interval")
	}
	for _, wantState := range []AuthResultState{AuthResultWaiting, AuthResultAuthorized} {
		poll, err := auth.PollAuthSessionStatus(context.Background(), begin.Session, time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if poll.State != wantState {
			t.Fatalf("poll state = %s, want %s", poll.State, wantState)
		}
		if wantState == AuthResultAuthorized && (poll.RefreshToken != "issued.refresh.token" || poll.AccessToken != "issued.access.token") {
			t.Fatal("issued tokens did not survive the login response")
		}
	}
}
