package enrollmentapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/protocol"
)

const testUnix = uint64(1784678400)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func testClient(transport http.RoundTripper, entropy io.Reader) *Client {
	if entropy == nil {
		entropy = bytes.NewReader(bytes.Repeat([]byte{0x41}, 64))
	}
	return newClientForTest(
		protocol.NewClient(protocol.Options{Transport: transport}),
		entropy,
		fixedClock{now: time.Unix(int64(testUnix), 0)},
	)
}

func response(status int, body []byte, header http.Header) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
	}
}

func validAddResponse(confirm uint64) []byte {
	shared := make([]byte, 20)
	identity := make([]byte, 20)
	secret1 := make([]byte, 20)
	for i := range shared {
		shared[i] = byte(i + 1)
		identity[i] = byte(i + 21)
		secret1[i] = byte(i + 41)
	}
	message := appendBytes(nil, 1, shared)
	message = appendFixed64(message, 2, 42)
	message = appendBytes(message, 3, []byte("R12345"))
	message = appendBytes(message, 4, []byte("otpauth://totp/Steam:test?secret=AAAA&issuer=Steam"))
	message = appendVarint(message, 5, testUnix)
	message = appendBytes(message, 6, []byte("test_account"))
	message = appendBytes(message, 7, []byte("123456789"))
	message = appendBytes(message, 8, identity)
	message = appendBytes(message, 9, secret1)
	message = appendVarint(message, 10, 1)
	message = appendBytes(message, 11, []byte("1234"))
	return appendVarint(message, 12, confirm)
}

func statusAddResponse(status int32) []byte {
	return appendVarint(nil, 10, uint64(status))
}

func testPending() *PendingEnrollment {
	return &PendingEnrollment{
		RequestID:      bytes.Repeat([]byte{0x31}, requestIDBytes),
		SteamID:        76561198000000001,
		AccessToken:    []byte("ACCESS-TOKEN-SENTINEL"),
		DeviceID:       "android:00010203-0405-4607-8809-0a0b0c0d0e0f",
		SharedSecret:   bytes.Repeat([]byte{0x11}, 20),
		IdentitySecret: bytes.Repeat([]byte{0x12}, 20),
		Secret1:        bytes.Repeat([]byte{0x13}, 20),
		RevocationCode: []byte("R12345"),
		URI:            []byte("otpauth://totp/Steam:test?secret=AAAA&issuer=Steam"),
		SerialNumber:   42,
		ServerTime:     testUnix,
		AccountName:    "test_account",
		TokenGID:       "123456789",
		PhoneHint:      "1234",
		Confirmation:   ConfirmationSMS,
	}
}

func TestAddAuthenticatorRequestAndResponseVector(t *testing.T) {
	entropy := make([]byte, 32)
	for i := range entropy {
		entropy[i] = byte(i)
	}
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != addAuthenticatorEndpoint || request.URL.RawQuery != "" {
			t.Fatalf("endpoint=%q", request.URL.String())
		}
		if request.Header.Get("Accept") != "application/x-protobuf" || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Fatalf("headers=%v", request.Header)
		}
		raw, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		form, err := url.ParseQuery(string(raw))
		if err != nil {
			t.Fatal(err)
		}
		if form.Get("access_token") != "ACCESS-TOKEN-SENTINEL" || len(form) != 2 {
			t.Fatalf("unexpected form keys")
		}
		message, err := base64.StdEncoding.DecodeString(form.Get("input_protobuf_encoded"))
		if err != nil {
			t.Fatal(err)
		}
		assertAddRequestVector(t, message)
		return response(http.StatusOK, validAddResponse(1), nil), nil
	})
	client := testClient(transport, bytes.NewReader(entropy))
	result, err := client.AddAuthenticator(context.Background(), AddRequest{
		SteamID: 76561198000000001, AccessToken: []byte("ACCESS-TOKEN-SENTINEL"), AuthenticatorTime: testUnix,
	}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != StateAwaitingSMS || result.Pending == nil {
		t.Fatalf("result=%+v", result)
	}
	pending := result.Pending
	if pending.DeviceID != "android:00010203-0405-4607-8809-0a0b0c0d0e0f" || !bytes.Equal(pending.RequestID, entropy[16:]) ||
		len(pending.SharedSecret) != 20 || len(pending.IdentitySecret) != 20 || pending.RevocationCode[0] != 'R' {
		t.Fatalf("pending metadata mismatch: %s", pending)
	}
	secretAlias := pending.SharedSecret
	pending.Destroy()
	if !bytes.Equal(secretAlias, make([]byte, len(secretAlias))) || pending.AccessToken != nil {
		t.Fatal("pending secrets were not wiped")
	}
}

func assertAddRequestVector(t *testing.T, message []byte) {
	t.Helper()
	decoder := wireDecoder{data: message}
	wantFields := []uint32{1, 2, 4, 5, 8}
	for index, want := range wantFields {
		field, ok := decoder.next()
		if !ok || field.number != want {
			t.Fatalf("field %d=%+v", index, field)
		}
		switch field.number {
		case 1:
			if field.fixed64 != 76561198000000001 {
				t.Fatalf("steamid=%d", field.fixed64)
			}
		case 2:
			if field.varint != testUnix {
				t.Fatalf("time=%d", field.varint)
			}
		case 4:
			if field.varint != 1 {
				t.Fatalf("type=%d", field.varint)
			}
		case 5:
			if string(field.bytes) != "android:00010203-0405-4607-8809-0a0b0c0d0e0f" {
				t.Fatalf("device=%q", field.bytes)
			}
		case 8:
			if field.varint != 2 {
				t.Fatalf("version=%d", field.varint)
			}
		}
	}
	if !decoder.validEnd() {
		t.Fatal("trailing add request data")
	}
}

func TestFinalizeAuthenticatorRequestAndStates(t *testing.T) {
	pending := testPending()
	defer pending.Destroy()
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != finalizeAuthenticatorEndpoint || request.URL.RawQuery != "" {
			t.Fatalf("endpoint=%q", request.URL.String())
		}
		raw, _ := io.ReadAll(request.Body)
		form, err := url.ParseQuery(string(raw))
		if err != nil {
			t.Fatal(err)
		}
		message, err := base64.StdEncoding.DecodeString(form.Get("input_protobuf_encoded"))
		if err != nil {
			t.Fatal(err)
		}
		assertFinalizeRequestVector(t, message)
		body := appendVarint(nil, 1, 1)
		body = appendVarint(body, 3, testUnix+1)
		body = appendVarint(body, 4, 1)
		return response(http.StatusOK, body, nil), nil
	})
	client := testClient(transport, nil)
	result, err := client.FinalizeAddAuthenticator(context.Background(), FinalizeRequest{
		Pending: pending, RequestID: append([]byte(nil), pending.RequestID...), ConfirmationCode: []byte("12345"), AuthenticatorTime: testUnix,
	}, time.Second)
	if err != nil || result.State != StateComplete || result.ServerTime != testUnix+1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func assertFinalizeRequestVector(t *testing.T, message []byte) {
	t.Helper()
	decoder := wireDecoder{data: message}
	wantFields := []uint32{1, 2, 3, 4, 6}
	for _, want := range wantFields {
		field, ok := decoder.next()
		if !ok || field.number != want {
			t.Fatalf("finalize field=%+v want=%d", field, want)
		}
		switch field.number {
		case 1:
			if field.fixed64 != 76561198000000001 {
				t.Fatalf("steamid=%d", field.fixed64)
			}
		case 2:
			if len(field.bytes) != 5 {
				t.Fatalf("authenticator code=%q", field.bytes)
			}
		case 3:
			if field.varint != testUnix {
				t.Fatalf("time=%d", field.varint)
			}
		case 4:
			if string(field.bytes) != "12345" {
				t.Fatalf("confirmation code=%q", field.bytes)
			}
		case 6:
			if field.varint != 1 {
				t.Fatalf("validate=%d", field.varint)
			}
		}
	}
	if !decoder.validEnd() {
		t.Fatal("trailing finalize request data")
	}
}

func TestAddAuthenticatorExpectedStates(t *testing.T) {
	tests := []struct {
		status int32
		state  State
	}{
		{2, StatePhoneRequired}, {123, StatePhoneRequired}, {29, StateAlreadyHasAuthenticator},
		{74, StateAwaitingEmail}, {84, StateRateLimited}, {77, StateReauthenticationRequired},
	}
	for _, test := range tests {
		t.Run(string(test.state), func(t *testing.T) {
			transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
				return response(http.StatusOK, statusAddResponse(test.status), nil), nil
			})
			result, err := testClient(transport, nil).AddAuthenticator(context.Background(), AddRequest{
				SteamID: 76561198000000001, AccessToken: []byte("token"), AuthenticatorTime: testUnix,
			}, time.Second)
			if err != nil || result.State != test.state || result.Pending != nil {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}

func TestFinalizeExpectedStates(t *testing.T) {
	tests := []struct {
		status   int32
		wantMore bool
		state    State
	}{
		{88, false, StateAuthenticatorCodeRetry}, {89, false, StateConfirmationCodeRejected},
		{84, false, StateRateLimited}, {77, false, StateReauthenticationRequired},
		{1, true, StateAuthenticatorCodeRetry},
	}
	for _, test := range tests {
		t.Run(string(test.state), func(t *testing.T) {
			body := appendVarint(nil, 1, 0)
			if test.wantMore {
				body = appendVarint(body, 2, 1)
			}
			body = appendVarint(body, 3, testUnix+1)
			body = appendVarint(body, 4, uint64(test.status))
			transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
				return response(http.StatusOK, body, nil), nil
			})
			pending := testPending()
			defer pending.Destroy()
			result, err := testClient(transport, nil).FinalizeAddAuthenticator(context.Background(), FinalizeRequest{
				Pending: pending, RequestID: pending.RequestID, ConfirmationCode: []byte("12345"), AuthenticatorTime: testUnix,
			}, time.Second)
			if err != nil || result.State != test.state {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}

func TestFinalizeSteamStatusOutranksContradictorySuccess(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := appendVarint(nil, 1, 1)
		body = appendVarint(body, 2, 1)
		body = appendVarint(body, 4, 89)
		return response(http.StatusOK, body, nil), nil
	})
	client := testClient(transport, bytes.NewReader(bytes.Repeat([]byte{0x33}, 32)))
	pending := testPending()
	defer pending.Destroy()

	result, err := client.FinalizeAddAuthenticator(context.Background(), FinalizeRequest{
		Pending: pending, RequestID: append([]byte(nil), pending.RequestID...),
		ConfirmationCode: []byte("ABCDE"), AuthenticatorTime: testUnix,
	}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != StateConfirmationCodeRejected {
		t.Fatalf("state = %q", result.State)
	}
}

func TestFinalizeRejectsTamperedPendingSecretMaterial(t *testing.T) {
	called := false
	client := testClient(roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return response(http.StatusOK, nil, nil), nil
	}), bytes.NewReader(bytes.Repeat([]byte{0x33}, 32)))
	pending := testPending()
	defer pending.Destroy()
	pending.IdentitySecret[0] ^= 0xff
	pending.IdentitySecret = pending.IdentitySecret[:19]

	_, err := client.FinalizeAddAuthenticator(context.Background(), FinalizeRequest{
		Pending: pending, RequestID: append([]byte(nil), pending.RequestID...),
		ConfirmationCode: []byte("ABCDE"), AuthenticatorTime: testUnix,
	}, time.Second)
	if !errors.Is(err, ErrInvalidRequest) || called {
		t.Fatalf("err = %v, called = %v", err, called)
	}
}

func TestTransportRateLimitAndReauthenticationStates(t *testing.T) {
	tests := []struct {
		status int
		state  State
	}{
		{http.StatusTooManyRequests, StateRateLimited},
		{http.StatusUnauthorized, StateReauthenticationRequired},
		{http.StatusForbidden, StateReauthenticationRequired},
	}
	for _, test := range tests {
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
				return response(test.status, nil, http.Header{"Retry-After": {"7"}}), nil
			})
			result, err := testClient(transport, nil).AddAuthenticator(context.Background(), AddRequest{
				SteamID: 76561198000000001, AccessToken: []byte("token"), AuthenticatorTime: testUnix,
			}, time.Second)
			if err != nil || result.State != test.state {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if test.status == http.StatusTooManyRequests && (!result.HasRetryAfter || result.RetryAfter != 7*time.Second) {
				t.Fatalf("retry metadata=%+v", result)
			}
		})
	}
}

func TestCancellationTimeoutMalformedOversizeAndSecretFreeErrors(t *testing.T) {
	t.Run("canceled", func(t *testing.T) {
		transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()
			return nil, request.Context().Err()
		})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := testClient(transport, nil).AddAuthenticator(ctx, AddRequest{
			SteamID: 76561198000000001, AccessToken: []byte("token"), AuthenticatorTime: testUnix,
		}, time.Second)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()
			return nil, request.Context().Err()
		})
		_, err := testClient(transport, nil).AddAuthenticator(context.Background(), AddRequest{
			SteamID: 76561198000000001, AccessToken: []byte("token"), AuthenticatorTime: testUnix,
		}, time.Millisecond)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("malformed secret free", func(t *testing.T) {
		const secret = "RESPONSE-SECRET-SENTINEL"
		transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
			return response(http.StatusOK, []byte(secret), nil), nil
		})
		_, err := testClient(transport, nil).AddAuthenticator(context.Background(), AddRequest{
			SteamID: 76561198000000001, AccessToken: []byte("token"), AuthenticatorTime: testUnix,
		}, time.Second)
		if !errors.Is(err, ErrInvalidResponse) || strings.Contains(err.Error(), secret) {
			t.Fatalf("error=%q", err)
		}
	})

	t.Run("oversize", func(t *testing.T) {
		transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
			result := response(http.StatusOK, nil, nil)
			result.ContentLength = maxResponseBytes + 1
			return result, nil
		})
		_, err := testClient(transport, nil).AddAuthenticator(context.Background(), AddRequest{
			SteamID: 76561198000000001, AccessToken: []byte("token"), AuthenticatorTime: testUnix,
		}, time.Second)
		var protocolErr *protocol.Error
		if !errors.As(err, &protocolErr) || protocolErr.Code != protocol.CodeResponseTooLarge {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("transport secret free", func(t *testing.T) {
		const secret = "TRANSPORT-SECRET-SENTINEL"
		transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New(secret)
		})
		_, err := testClient(transport, nil).AddAuthenticator(context.Background(), AddRequest{
			SteamID: 76561198000000001, AccessToken: []byte(secret), AuthenticatorTime: testUnix,
		}, time.Second)
		if err == nil || strings.Contains(err.Error(), secret) {
			t.Fatalf("error=%q", err)
		}
	})
}

func TestEntropyAndInputBoundsFailBeforeTransport(t *testing.T) {
	called := false
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return response(http.StatusOK, nil, nil), nil
	})
	client := testClient(transport, bytes.NewReader(nil))
	_, err := client.AddAuthenticator(context.Background(), AddRequest{
		SteamID: 76561198000000001, AccessToken: []byte("token"), AuthenticatorTime: testUnix,
	}, time.Second)
	if !errors.Is(err, ErrEntropy) || called {
		t.Fatalf("entropy error=%v called=%v", err, called)
	}

	pending := testPending()
	defer pending.Destroy()
	client = testClient(transport, nil)
	_, err = client.FinalizeAddAuthenticator(context.Background(), FinalizeRequest{
		Pending: pending, RequestID: []byte("wrong"), ConfirmationCode: []byte("12 45"), AuthenticatorTime: testUnix,
	}, time.Second)
	if !errors.Is(err, ErrInvalidRequest) || called {
		t.Fatalf("validation error=%v called=%v", err, called)
	}
}
