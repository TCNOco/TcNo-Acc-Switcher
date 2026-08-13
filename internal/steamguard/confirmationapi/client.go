package confirmationapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/steamguard/protocol"
)

const (
	maxListResponseBytes     = 512 << 10
	maxDetailsResponseBytes  = 512 << 10
	maxDecisionResponseBytes = 16 << 10
)

// Clock supplies the signature timestamp.
type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Options exposes only deterministic test dependencies.
type Options struct {
	Protocol *protocol.Client
	Clock    Clock
	Offline  func() bool
}

// Client executes fixed Steam Community confirmation operations.
type Client struct {
	protocol *protocol.Client
	clock    Clock
	offline  func() bool
}

func NewClient(options Options) *Client {
	transport := options.Protocol
	if transport == nil {
		transport = protocol.NewClient(protocol.Options{})
	}
	clock := options.Clock
	if clock == nil {
		clock = systemClock{}
	}
	offline := options.Offline
	if offline == nil {
		offline = appclient.IsOfflineMode
	}
	return &Client{protocol: transport, clock: clock, offline: offline}
}

// clientLogger records transport status and response sizes only. Response
// bodies carry trade and account details, so they are never logged.
func clientLogger() *slog.Logger {
	return slog.Default().With("component", "steamguard.confirmations")
}

// logTransportFailure records the HTTP status behind a classified failure.
func logTransportFailure(operation string, err error) {
	attributes := []any{"operation", operation}
	var apiErr *Error
	if errors.As(err, &apiErr) {
		attributes = append(attributes, "kind", string(apiErr.Kind))
		if apiErr.StatusCode != 0 {
			attributes = append(attributes, "status", apiErr.StatusCode)
		}
		// Without this, a refused redirect and a refused session are the same
		// line: kind=reauth, no status, nothing to tell them apart.
		if apiErr.Detail != "" {
			attributes = append(attributes, "detail", apiErr.Detail)
		}
	}
	clientLogger().Warn("mobileconf request failed", append(attributes, "error", err)...)
}

// logDecodeFailure records the response size, never its content. A needauth
// envelope is a normal auth outcome rather than a broken response, so it is
// reported as such instead of as a decode failure.
func logDecodeFailure(operation string, response protocol.Response, err error) {
	var apiErr *Error
	if errors.As(err, &apiErr) && apiErr.Kind == FailureReauth {
		clientLogger().Info("mobileconf session needs re-authentication",
			"operation", operation, "status", response.StatusCode, "bodyBytes", len(response.Body))
		return
	}
	clientLogger().Warn("mobileconf response could not be decoded",
		"operation", operation, "status", response.StatusCode, "bodyBytes", len(response.Body), "error", err)
}

func (c *Client) CloseIdleConnections() {
	if c != nil && c.protocol != nil {
		c.protocol.CloseIdleConnections()
	}
}

// List fetches a fresh snapshot. It keeps no stale rows and produces stable
// opaque handles from Steam's decision identity.
func (c *Client) List(ctx context.Context, credentials Credentials) ([]Confirmation, error) {
	request, err := c.signedRequest(ctx, credentials, http.MethodGet, ListEndpoint, "conf", nil, maxListResponseBytes)
	if err != nil {
		return nil, err
	}
	response, err := c.protocol.Do(ctx, request)
	if err != nil {
		classified := classifyProtocolError(err)
		logTransportFailure("getlist", classified)
		return nil, classified
	}
	confirmations, decodeErr := decodeList(response.Body)
	if decodeErr != nil {
		logDecodeFailure("getlist", response, decodeErr)
		return nil, decodeErr
	}
	return confirmations, nil
}

// FetchDetails converts Steam's HTML response into bounded plain-text fields.
func (c *Client) FetchDetails(ctx context.Context, credentials Credentials, confirmation Confirmation) (Details, error) {
	if !validReference(confirmation) {
		return Details{}, &Error{Kind: FailureInvalid}
	}
	endpoint := DetailsBase + url.PathEscape(confirmation.id)
	// Steam otherwise renders this page in the account's own language, and the
	// listing parser reads counts out of English wording. Nothing off this page
	// reaches the window as prose, so pinning it costs the user nothing.
	extra := url.Values{"l": {"english"}}
	request, err := c.signedRequest(ctx, credentials, http.MethodGet, endpoint, "details", extra, maxDetailsResponseBytes)
	if err != nil {
		return Details{}, err
	}
	response, err := c.protocol.Do(ctx, request)
	if err != nil {
		classified := classifyProtocolError(err)
		logTransportFailure("details", classified)
		return Details{}, classified
	}
	fields, trade, listing, err := decodeDetails(response.Body)
	if err != nil {
		logDecodeFailure("details", response, err)
		return Details{}, err
	}
	return Details{
		Handle: confirmation.item.Handle, Fields: fields,
		Trade: trade, Listing: listing, Raw: response.Body,
	}, nil
}

// Decide allows or denies one confirmation using its private Steam keys.
func (c *Client) Decide(ctx context.Context, credentials Credentials, confirmation Confirmation, decision Decision) error {
	if !validReference(confirmation) || (decision != Allow && decision != Deny) {
		return &Error{Kind: FailureInvalid}
	}
	tag, operation := "accept", "allow"
	if decision == Deny {
		tag, operation = "reject", "cancel"
	}
	extra := url.Values{"op": {operation}, "cid": {confirmation.id}, "ck": {confirmation.nonce}}
	request, err := c.signedRequest(ctx, credentials, http.MethodGet, DecisionEndpoint, tag, extra, maxDecisionResponseBytes)
	if err != nil {
		return err
	}
	response, err := c.protocol.Do(ctx, request)
	if err != nil {
		classified := classifyProtocolError(err)
		logTransportFailure(operation, classified)
		return classified
	}
	if decisionErr := decodeDecision(response.Body); decisionErr != nil {
		// Steam answers 200 with success=false or needauth, so this covers both a
		// rejected decision and an undecodable envelope.
		var apiErr *Error
		kind := FailureFailed
		if errors.As(decisionErr, &apiErr) {
			kind = apiErr.Kind
		}
		clientLogger().Warn("mobileconf decision was not accepted", "operation", operation,
			"status", response.StatusCode, "bodyBytes", len(response.Body), "kind", string(kind))
		return decisionErr
	}
	return nil
}

// DecideBatch allows or denies several confirmations as one request, which is
// how the Steam app submits a multi-select. One request matters beyond the
// round trips: the signature is built from the tag and the current second, so
// two single decisions fired back to back sign identically and Steam rejects
// the second as a replay.
func (c *Client) DecideBatch(ctx context.Context, credentials Credentials, confirmations []Confirmation, decision Decision) error {
	if len(confirmations) == 0 || (decision != Allow && decision != Deny) {
		return &Error{Kind: FailureInvalid}
	}
	if len(confirmations) == 1 {
		return c.Decide(ctx, credentials, confirmations[0], decision)
	}
	for _, confirmation := range confirmations {
		if !validReference(confirmation) {
			return &Error{Kind: FailureInvalid}
		}
	}
	tag, operation := "accept", "allow"
	if decision == Deny {
		tag, operation = "reject", "cancel"
	}
	extra := url.Values{"op": {operation}}
	for _, confirmation := range confirmations {
		extra.Add("cid[]", confirmation.id)
		extra.Add("ck[]", confirmation.nonce)
	}
	request, err := c.signedRequest(ctx, credentials, http.MethodPost, MultiDecisionEndpoint, tag, extra, maxDecisionResponseBytes)
	if err != nil {
		return err
	}
	response, err := c.protocol.Do(ctx, request)
	if err != nil {
		classified := classifyProtocolError(err)
		logTransportFailure("multi-"+operation, classified)
		return classified
	}
	if decisionErr := decodeDecision(response.Body); decisionErr != nil {
		var apiErr *Error
		kind := FailureFailed
		if errors.As(decisionErr, &apiErr) {
			kind = apiErr.Kind
		}
		clientLogger().Warn("mobileconf decision was not accepted", "operation", "multi-"+operation,
			"status", response.StatusCode, "bodyBytes", len(response.Body), "kind", string(kind))
		return decisionErr
	}
	return nil
}

func (c *Client) signedRequest(ctx context.Context, credentials Credentials, method, endpoint, tag string, extra url.Values, limit int64) (protocol.Request, error) {
	if c == nil || c.protocol == nil || ctx == nil || c.offline == nil || c.clock == nil {
		return protocol.Request{}, &Error{Kind: FailureInvalid}
	}
	if c.offline() {
		return protocol.Request{}, &Error{Kind: FailureOffline}
	}
	if err := validateCredentials(credentials); err != nil {
		return protocol.Request{}, err
	}
	now := c.clock.Now().Unix()
	signature, err := GenerateConfirmationHash(credentials.IdentitySecret, now, tag)
	if err != nil {
		return protocol.Request{}, err
	}
	query := url.Values{
		"p":   {credentials.DeviceID},
		"a":   {credentials.SteamID},
		"k":   {signature},
		"t":   {strconv.FormatInt(now, 10)},
		"m":   {"react"},
		"tag": {tag},
	}
	for key, values := range extra {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	parsed, parseErr := url.Parse(endpoint)
	if parseErr != nil || parsed.Scheme != "https" || parsed.Host != "steamcommunity.com" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return protocol.Request{}, &Error{Kind: FailureInvalid}
	}
	// Steam only answers mobileconf for the mobile app fingerprint: the okhttp
	// User-Agent and the cookies below, with no browser-style Accept, Origin,
	// Referer or X-Requested-With headers.
	headers := make(http.Header)
	headers.Set("User-Agent", MobileUserAgent)
	headers.Set("Cookie", confirmationCookie(credentials))
	// A POST (multiajaxop) carries the signed values as its form body, the way
	// the Steam app sends it; everything else keeps them in the query string.
	var body []byte
	if method == http.MethodPost {
		body = []byte(query.Encode())
		headers.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		parsed.RawQuery = query.Encode()
	}
	return protocol.Request{
		Method: method, Endpoint: parsed.String(), Route: protocol.RouteRequest,
		Header: headers, Body: body, Timeout: RequestTimeout, MaxResponseBytes: limit,
	}, nil
}

func confirmationCookie(credentials Credentials) string {
	return "steamLoginSecure=" + credentials.SteamID + "%7C%7C" + credentials.AccessToken +
		"; sessionid=" + credentials.SessionID +
		"; mobileClient=android; mobileClientVersion=777777%203.6.4"
}

func validateCredentials(credentials Credentials) error {
	steamID, err := strconv.ParseUint(credentials.SteamID, 10, 64)
	if err != nil || steamID == 0 || strconv.FormatUint(steamID, 10) != credentials.SteamID {
		return &Error{Kind: FailureInvalid}
	}
	if len(credentials.DeviceID) < 9 || len(credentials.DeviceID) > 128 || !strings.HasPrefix(credentials.DeviceID, "android:") || !safeToken(credentials.DeviceID, true) {
		return &Error{Kind: FailureInvalid}
	}
	if len(credentials.AccessToken) < 16 || len(credentials.AccessToken) > 4096 || !safeToken(credentials.AccessToken, false) {
		return &Error{Kind: FailureInvalid}
	}
	if len(credentials.SessionID) < 16 || len(credentials.SessionID) > 64 || !isASCIIHex(credentials.SessionID) {
		return &Error{Kind: FailureInvalid}
	}
	_, err = GenerateConfirmationHash(credentials.IdentitySecret, 1, "conf")
	return err
}

func safeToken(value string, allowColon bool) bool {
	for i := 0; i < len(value); i++ {
		char := value[i]
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' || char == '.' || (allowColon && char == ':') {
			continue
		}
		return false
	}
	return true
}

func isASCIIHex(value string) bool {
	for i := 0; i < len(value); i++ {
		char := value[i]
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}

func validReference(confirmation Confirmation) bool {
	return confirmation.item.Handle != "" && validDecimal(confirmation.id) && validDecimal(confirmation.nonce)
}

func stableHandle(id string) string {
	digest := sha256.Sum256([]byte(id))
	return base64.RawURLEncoding.EncodeToString(digest[:24])
}

func classifyProtocolError(err error) error {
	var protocolErr *protocol.Error
	if !errors.As(err, &protocolErr) {
		return &Error{Kind: FailureFailed}
	}
	result := &Error{
		StatusCode: protocolErr.StatusCode, RetryAfter: protocolErr.RetryAfter,
		HasRetryAfter: protocolErr.HasRetryAfter, Detail: protocolErr.Detail,
	}
	switch {
	case protocolErr.Code == protocol.CodeCanceled:
		result.Kind = FailureCanceled
	case protocolErr.Code == protocol.CodeDeadlineExceeded:
		result.Kind = FailureTimeout
	case protocolErr.Code == protocol.CodeRateLimited || protocolErr.StatusCode == http.StatusTooManyRequests:
		result.Kind = FailureRateLimit
	case protocolErr.Code == protocol.CodeRedirectDenied ||
		protocolErr.StatusCode == http.StatusUnauthorized || protocolErr.StatusCode == http.StatusForbidden ||
		(protocolErr.StatusCode >= http.StatusMultipleChoices && protocolErr.StatusCode < http.StatusBadRequest):
		result.Kind = FailureReauth
	case protocolErr.State == protocol.StateRetryable:
		result.Kind = FailureRetryable
	case protocolErr.State == protocol.StateInvalid || protocolErr.State == protocol.StateDenied:
		result.Kind = FailureInvalid
	default:
		result.Kind = FailureFailed
	}
	return result
}

func validDecimal(value string) bool {
	parsed, err := strconv.ParseUint(value, 10, 64)
	return err == nil && parsed > 0 && strconv.FormatUint(parsed, 10) == value
}
