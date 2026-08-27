package enrollmentapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/protocol"
)

const (
	addAuthenticatorEndpoint      = "https://api.steampowered.com/ITwoFactorService/AddAuthenticator/v1"
	finalizeAuthenticatorEndpoint = "https://api.steampowered.com/ITwoFactorService/FinalizeAddAuthenticator/v1"
)

type Client struct {
	protocol *protocol.Client
	entropy  io.Reader
	clock    clock
}

func NewClient(client *protocol.Client) *Client {
	return &Client{protocol: client, entropy: rand.Reader, clock: systemClock{}}
}

func newClientForTest(client *protocol.Client, entropy io.Reader, clock clock) *Client {
	return &Client{protocol: client, entropy: entropy, clock: clock}
}

func (c *Client) AddAuthenticator(ctx context.Context, request AddRequest, timeout time.Duration) (AddResult, error) {
	if c == nil || c.protocol == nil || c.entropy == nil || c.clock == nil || ctx == nil ||
		!validSteamID(request.SteamID) || !validToken(request.AccessToken) ||
		!validTimestamp(request.AuthenticatorTime, c.clock.Now()) || timeout <= 0 || timeout > requestTimeout {
		return AddResult{}, ErrInvalidRequest
	}
	deviceID, err := c.newDeviceID()
	if err != nil {
		return AddResult{}, err
	}
	requestID, err := c.randomBytes(requestIDBytes)
	if err != nil {
		return AddResult{}, err
	}
	message := marshalAddRequest(request.SteamID, request.AuthenticatorTime, deviceID)
	response, callErr := c.call(ctx, addAuthenticatorEndpoint, request.AccessToken, message, timeout)
	wipe(message)
	if callErr != nil {
		wipe(requestID)
		if state, retryAfter, hasRetryAfter, ok := expectedTransportState(callErr); ok {
			return AddResult{State: state, RetryAfter: retryAfter, HasRetryAfter: hasRetryAfter}, nil
		}
		return AddResult{}, callErr
	}
	defer wipe(response)
	wireResult, parseErr := unmarshalAddResponse(response)
	if parseErr != nil {
		wipe(requestID)
		return AddResult{}, parseErr
	}
	state := addStateForStatus(wireResult.status)
	if state != "" {
		wipe(requestID)
		return AddResult{State: state}, nil
	}
	if wireResult.status != 1 || wireResult.pending == nil {
		wipe(requestID)
		return AddResult{}, &SteamError{ResultCode: wireResult.status}
	}
	pending := wireResult.pending
	pending.RequestID = requestID
	pending.SteamID = request.SteamID
	pending.AccessToken = append([]byte(nil), request.AccessToken...)
	pending.DeviceID = deviceID
	switch pending.Confirmation {
	case ConfirmationSMS:
		state = StateAwaitingSMS
	case ConfirmationEmail:
		state = StateAwaitingEmail
	default:
		state = StateAwaitingConfirmation
	}
	return AddResult{State: state, Pending: pending}, nil
}

func (c *Client) FinalizeAddAuthenticator(ctx context.Context, request FinalizeRequest, timeout time.Duration) (FinalizeResult, error) {
	if c == nil || c.protocol == nil || c.clock == nil || ctx == nil || timeout <= 0 || timeout > requestTimeout ||
		!validPending(request.Pending, request.RequestID, c.clock.Now()) || !validConfirmationCode(request.ConfirmationCode) ||
		!validTimestamp(request.AuthenticatorTime, c.clock.Now()) {
		return FinalizeResult{}, ErrInvalidRequest
	}
	code, err := authenticatorCode(request.Pending.SharedSecret, request.AuthenticatorTime)
	if err != nil {
		return FinalizeResult{}, err
	}
	defer wipe(code)
	message := marshalFinalizeRequest(request.Pending.SteamID, code, request.AuthenticatorTime, request.ConfirmationCode,
		request.Pending.Confirmation == ConfirmationSMS)
	response, callErr := c.call(ctx, finalizeAuthenticatorEndpoint, request.Pending.AccessToken, message, timeout)
	wipe(message)
	if callErr != nil {
		if state, retryAfter, hasRetryAfter, ok := expectedTransportState(callErr); ok {
			return FinalizeResult{State: state, RetryAfter: retryAfter, HasRetryAfter: hasRetryAfter}, nil
		}
		return FinalizeResult{}, callErr
	}
	defer wipe(response)
	wireResult, parseErr := unmarshalFinalizeResponse(response)
	if parseErr != nil {
		return FinalizeResult{}, parseErr
	}
	result := FinalizeResult{ServerTime: wireResult.serverTime}
	if wireResult.status != 1 {
		if state := finalizeStateForStatus(wireResult.status); state != "" {
			result.State = state
			return result, nil
		}
		return FinalizeResult{}, &SteamError{ResultCode: wireResult.status}
	}
	// status 1 is Steam's OK and is authoritative: the authenticator exists on
	// the account by this point. success is not always set alongside it, so it
	// is ignored; want_more is absent from the current response, and absent has
	// to read as done rather than as a rejection.
	if wireResult.wantMore {
		result.State = StateAuthenticatorCodeRetry
		return result, nil
	}
	result.State = StateComplete
	return result, nil
}

func (c *Client) call(ctx context.Context, endpoint string, token, message []byte, timeout time.Duration) ([]byte, error) {
	body := encodeForm(token, message)
	defer wipe(body)
	response, err := c.protocol.Do(ctx, protocol.Request{
		Method:   http.MethodPost,
		Endpoint: endpoint,
		Route:    protocol.RouteRequest,
		Header: http.Header{
			"Accept":       {"application/x-protobuf"},
			"Content-Type": {"application/x-www-form-urlencoded"},
		},
		Body:             body,
		Timeout:          timeout,
		MaxResponseBytes: maxResponseBytes,
	})
	if err != nil {
		return nil, err
	}
	return response.Body, nil
}

func (c *Client) newDeviceID() (string, error) {
	value, err := c.randomBytes(16)
	if err != nil {
		return "", err
	}
	defer wipe(value)
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("android:%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}

func (c *Client) randomBytes(size int) ([]byte, error) {
	value := make([]byte, size)
	if _, err := io.ReadFull(c.entropy, value); err != nil {
		wipe(value)
		return nil, ErrEntropy
	}
	return value, nil
}

func encodeForm(token, message []byte) []byte {
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(message)))
	base64.StdEncoding.Encode(encoded, message)
	body := make([]byte, 0, len(token)+len(encoded)+64)
	body = append(body, "access_token="...)
	body = appendEscaped(body, token)
	body = append(body, "&input_protobuf_encoded="...)
	body = appendEscaped(body, encoded)
	wipe(encoded)
	return body
}

func appendEscaped(dst, value []byte) []byte {
	const hex = "0123456789ABCDEF"
	for _, b := range value {
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '-' || b == '_' || b == '.' || b == '~' {
			dst = append(dst, b)
			continue
		}
		dst = append(dst, '%', hex[b>>4], hex[b&15])
	}
	return dst
}

func expectedTransportState(err error) (State, time.Duration, bool, bool) {
	var protocolErr *protocol.Error
	if !errors.As(err, &protocolErr) {
		return "", 0, false, false
	}
	if protocolErr.Code == protocol.CodeRateLimited || protocolErr.StatusCode == http.StatusTooManyRequests {
		return StateRateLimited, protocolErr.RetryAfter, protocolErr.HasRetryAfter, true
	}
	if protocolErr.StatusCode == http.StatusUnauthorized || protocolErr.StatusCode == http.StatusForbidden {
		return StateReauthenticationRequired, 0, false, true
	}
	return "", 0, false, false
}

func addStateForStatus(status int32) State {
	switch status {
	case 2, 123:
		return StatePhoneRequired
	case 29:
		return StateAlreadyHasAuthenticator
	case 74:
		return StateAwaitingEmail
	case 84, 95, 96, 97:
		return StateRateLimited
	case 5, 15, 26, 27, 65, 77, 126:
		return StateReauthenticationRequired
	default:
		return ""
	}
}

func finalizeStateForStatus(status int32) State {
	switch status {
	case 88, 93:
		return StateAuthenticatorCodeRetry
	case 89, 94:
		return StateConfirmationCodeRejected
	case 84, 95, 96, 97:
		return StateRateLimited
	case 5, 15, 26, 27, 65, 77, 126:
		return StateReauthenticationRequired
	default:
		return ""
	}
}
