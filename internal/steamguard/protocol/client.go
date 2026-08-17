// Package protocol provides the bounded HTTP boundary used by Steam Guard
// protocol implementations.
package protocol

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	MaxRequestBodyBytes  = 1 << 20
	MaxResponseBodyBytes = 4 << 20
	MaxRequestTimeout    = 2 * time.Minute
	MaxRedirects         = 3
	// MaxRedirectBudget is the most a request may raise MaxRedirects to. Steam
	// reaches an account settings page through several handoffs a browser never
	// shows - a vanity canonicalisation, a session hop, a bounce onto the login
	// host - and three is short enough to refuse a chain that was going to
	// settle. Still far under a browser's twenty.
	MaxRedirectBudget = 10
	UserAgent         = "TcNo Account Switcher"
)

// Options permits transport replacement for tests. Policy and size limits are
// fixed and cannot be loosened by a caller.
type Options struct {
	Transport http.RoundTripper
}

// Client executes bounded requests against fixed official Steam hosts.
type Client struct {
	transport http.RoundTripper
}

// Request contains one Steam HTTP operation. Timeout is required for every
// request. MaxResponseBytes may reduce, but never raise, the package limit.
type Request struct {
	Method           string
	Endpoint         string
	Route            Route
	Header           http.Header
	Body             []byte
	Timeout          time.Duration
	MaxResponseBytes int64
	AllowRedirects   bool
	// PreserveHeadersOnRedirect carries the request's own headers - including
	// Cookie - onto a redirect that stays on the exact host the request started
	// on. Off by default, and ignored without AllowRedirects.
	//
	// It exists for Steam's own canonicalising redirects: an authenticated GET of
	// /profiles/<id64>/... is answered with a 302 to /id/<vanity>/... for any
	// account with a custom URL, and a followed request stripped of its cookies
	// lands on the login page. Same host is the whole guarantee - a redirect one
	// hop sideways to another allowlisted Steam host still gets the scrubbed
	// headers, so a session cookie can never be replayed to an origin that did
	// not issue it.
	PreserveHeadersOnRedirect bool
	// MaxRedirects raises this request's hop budget, up to MaxRedirectBudget.
	// Zero selects the package default. Ignored without AllowRedirects.
	MaxRedirects int
	// OnRedirect observes each hop this follows, as scheme://host/path with the
	// query removed. It runs only for hops that passed every policy check, so it
	// never names a destination this refused - the error says that one was, and
	// which check did it.
	//
	// Called synchronously from the redirect check, so it must not block. For a
	// caller that has to see where a chain went; nil disables it.
	OnRedirect func(hop int, target string)
}

// Response omits headers except for parsed Retry-After and Steam EResult
// metadata.
type Response struct {
	StatusCode    int
	Body          []byte
	EResult       int
	HasEResult    bool
	RetryAfter    time.Duration
	HasRetryAfter bool
}

// NewClient creates a client with the hardened transport unless a test
// transport is supplied.
func NewClient(options Options) *Client {
	transport := options.Transport
	if transport == nil {
		transport = NewTransport()
	}
	return &Client{transport: transport}
}

// NewTransport returns the production transport. It ignores proxy environment
// variables so authentication traffic cannot be redirected by process state.
func NewTransport() *http.Transport {
	return &http.Transport{
		Proxy:                  nil,
		DialContext:            (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:      true,
		// Sized for the sweeps rather than for a single request: the CS2 sweep now
		// reads several accounts at once and the owned games sweep runs alongside
		// it, all against steamcommunity.com. At two idle slots most of those paid
		// for a fresh TCP and TLS handshake. This bounds pooling, not policy - what
		// the transport is allowed to talk to is decided above and below it.
		MaxIdleConns:           16,
		MaxIdleConnsPerHost:    6,
		IdleConnTimeout:        30 * time.Second,
		TLSHandshakeTimeout:    10 * time.Second,
		ResponseHeaderTimeout:  15 * time.Second,
		ExpectContinueTimeout:  time.Second,
		MaxResponseHeaderBytes: 64 << 10,
		DisableCompression:     true,
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
	}
}

// CloseIdleConnections closes pooled connections when the owning session ends.
func (c *Client) CloseIdleConnections() {
	if c == nil || c.transport == nil {
		return
	}
	if closer, ok := c.transport.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

// Do validates, executes, and fully reads one bounded response.
func (c *Client) Do(ctx context.Context, request Request) (Response, error) {
	if c == nil || c.transport == nil || ctx == nil {
		return Response{}, protocolError(CodeInvalidRequest, StateInvalid)
	}
	if request.Timeout <= 0 || request.Timeout > MaxRequestTimeout {
		return Response{}, protocolError(CodeInvalidRequest, StateInvalid)
	}
	if request.Method != http.MethodGet && request.Method != http.MethodPost && request.Method != http.MethodHead {
		return Response{}, protocolError(CodeInvalidRequest, StateInvalid)
	}
	if (request.Method == http.MethodGet || request.Method == http.MethodHead) && len(request.Body) != 0 {
		return Response{}, protocolError(CodeInvalidRequest, StateInvalid)
	}
	if len(request.Body) > MaxRequestBodyBytes {
		return Response{}, protocolError(CodeRequestTooLarge, StateInvalid)
	}
	limit := request.MaxResponseBytes
	if limit == 0 {
		limit = MaxResponseBodyBytes
	}
	if limit < 0 || limit > MaxResponseBodyBytes {
		return Response{}, protocolError(CodeInvalidRequest, StateInvalid)
	}
	budget := MaxRedirects
	if request.MaxRedirects != 0 {
		if request.MaxRedirects < 0 || request.MaxRedirects > MaxRedirectBudget {
			return Response{}, protocolError(CodeInvalidRequest, StateInvalid)
		}
		budget = request.MaxRedirects
	}

	endpoint, policyErr := validateEndpoint(request.Endpoint, request.Route)
	if policyErr != nil {
		return Response{}, policyErr
	}

	requestCtx, cancel := context.WithTimeout(ctx, request.Timeout)
	defer cancel()

	var body io.Reader
	if len(request.Body) != 0 {
		body = bytes.NewReader(request.Body)
	}
	httpRequest, err := http.NewRequestWithContext(requestCtx, request.Method, endpoint.String(), body)
	if err != nil {
		return Response{}, protocolError(CodeInvalidRequest, StateInvalid)
	}
	httpRequest.Header = request.Header.Clone()
	if httpRequest.Header == nil {
		httpRequest.Header = make(http.Header)
	}
	// A caller that must match another client's fingerprint (mobile confirmations
	// send okhttp) may set User-Agent itself; everything else gets ours.
	userAgent := httpRequest.Header.Get("User-Agent")
	if userAgent == "" {
		userAgent = UserAgent
	}
	httpRequest.Header.Set("User-Agent", userAgent)
	httpRequest.Header.Set("Accept-Encoding", "identity")
	// Read from the validated endpoint, not from a header a caller could set.
	originHost := strings.ToLower(endpoint.Hostname())

	// Only a chain that carries the caller's cookies has anywhere to put the ones
	// Steam sets along the way, so nothing else pays for the wrapper.
	transport := c.transport
	var carried *carriedCookies
	if request.AllowRedirects && request.PreserveHeadersOnRedirect {
		carried = newCarriedCookies(originHost, transport)
		transport = carried
	}

	httpClient := &http.Client{
		Transport: transport,
		CheckRedirect: func(next *http.Request, via []*http.Request) error {
			if !request.AllowRedirects {
				return redirectDenied("disabled")
			}
			if len(via) > budget {
				return redirectDenied("limit")
			}
			if next.Method != http.MethodGet && next.Method != http.MethodHead {
				return redirectDenied("method")
			}
			if redirectErr := validateRedirect(next.URL); redirectErr != nil {
				return redirectErr
			}
			if request.PreserveHeadersOnRedirect && sameHost(next.URL, originHost) {
				next.Header = httpRequest.Header.Clone()
				if cookie := carried.header(httpRequest.Header.Get("Cookie")); cookie != "" {
					next.Header.Set("Cookie", cookie)
				}
			} else {
				next.Header = make(http.Header)
				next.Header.Set("User-Agent", userAgent)
				next.Header.Set("Accept-Encoding", "identity")
			}
			if request.OnRedirect != nil {
				request.OnRedirect(len(via), redirectTarget(next.URL))
			}
			return nil
		},
	}

	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		if httpResponse != nil && httpResponse.Body != nil {
			_ = httpResponse.Body.Close()
		}
		return Response{}, classifyRequestError(requestCtx, err)
	}
	if httpResponse.Body == nil {
		return Response{}, protocolError(CodeResponseRead, StateRetryable)
	}
	defer httpResponse.Body.Close()

	retryAfter, hasRetryAfter := ParseRetryAfter(httpResponse.Header.Get("Retry-After"), time.Now())
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		code := CodeHTTPStatus
		if httpResponse.StatusCode == http.StatusTooManyRequests {
			code = CodeRateLimited
		}
		state := StateFailed
		if retryableStatus(httpResponse.StatusCode) {
			state = StateRetryable
		}
		return Response{}, &Error{
			Code:          code,
			State:         state,
			StatusCode:    httpResponse.StatusCode,
			RetryAfter:    retryAfter,
			HasRetryAfter: hasRetryAfter,
		}
	}
	result, hasResult, resultErr := parseEResult(httpResponse.Header)
	if resultErr != nil {
		return Response{}, resultErr
	}
	if httpResponse.ContentLength > limit {
		return Response{}, protocolError(CodeResponseTooLarge, StateFailed)
	}

	responseBody, err := io.ReadAll(io.LimitReader(httpResponse.Body, limit+1))
	if err != nil {
		return Response{}, classifyReadError(requestCtx)
	}
	if int64(len(responseBody)) > limit {
		return Response{}, protocolError(CodeResponseTooLarge, StateFailed)
	}
	return Response{
		StatusCode:    httpResponse.StatusCode,
		Body:          responseBody,
		EResult:       result,
		HasEResult:    hasResult,
		RetryAfter:    retryAfter,
		HasRetryAfter: hasRetryAfter,
	}, nil
}

// sameHost reports whether a redirect target is the very host the request
// started on. Hostname() drops any port, which validateEndpoint has already
// pinned to 443 or empty on both ends.
func sameHost(target *url.URL, originHost string) bool {
	return target != nil && strings.ToLower(target.Hostname()) == originHost && originHost != ""
}

func parseEResult(header http.Header) (int, bool, *Error) {
	var values []string
	for name, current := range header {
		if strings.EqualFold(name, "X-EResult") {
			values = append(values, current...)
		}
	}
	if len(values) == 0 {
		return 0, false, nil
	}
	if len(values) != 1 || values[0] == "" {
		return 0, false, &Error{Code: CodeInvalidResponse, State: StateFailed, Detail: "eresult_header_repeated_or_empty"}
	}
	parsed, err := strconv.ParseUint(values[0], 10, 31)
	if err != nil || parsed == 0 || parsed > math.MaxInt32 || strconv.FormatUint(parsed, 10) != values[0] {
		return 0, false, &Error{Code: CodeInvalidResponse, State: StateFailed, Detail: "eresult_header_malformed"}
	}
	return int(parsed), true, nil
}

func classifyRequestError(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return protocolError(CodeCanceled, StateCanceled)
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return protocolError(CodeDeadlineExceeded, StateRetryable)
	}
	var protocolErr *Error
	if errors.As(err, &protocolErr) {
		// Rebuilt rather than returned, so the transport error underneath - which
		// carries the URL - is left behind. Every field has to be carried across
		// by hand for that to be safe, and Detail was being dropped.
		return &Error{
			Code:          protocolErr.Code,
			State:         protocolErr.State,
			StatusCode:    protocolErr.StatusCode,
			EResult:       protocolErr.EResult,
			HasEResult:    protocolErr.HasEResult,
			RetryAfter:    protocolErr.RetryAfter,
			HasRetryAfter: protocolErr.HasRetryAfter,
			Detail:        protocolErr.Detail,
		}
	}
	return protocolError(CodeTransport, StateRetryable)
}

func classifyReadError(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		return protocolError(CodeCanceled, StateCanceled)
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return protocolError(CodeDeadlineExceeded, StateRetryable)
	}
	return protocolError(CodeResponseRead, StateRetryable)
}

func retryableStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout,
		http.StatusTooEarly,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
