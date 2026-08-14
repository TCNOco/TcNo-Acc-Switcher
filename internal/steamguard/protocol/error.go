package protocol

import (
	"context"
	"strconv"
	"time"
)

// Code is a stable, machine-readable protocol error identifier.
type Code string

const (
	CodeInvalidRequest   Code = "invalid_request"
	CodeSchemeDenied     Code = "scheme_denied"
	CodeHostDenied       Code = "host_denied"
	CodeRedirectDenied   Code = "redirect_denied"
	CodeRequestTooLarge  Code = "request_too_large"
	CodeCanceled         Code = "canceled"
	CodeDeadlineExceeded Code = "deadline_exceeded"
	CodeTransport        Code = "transport_failure"
	CodeResponseRead     Code = "response_read_failure"
	CodeResponseTooLarge Code = "response_too_large"
	CodeHTTPStatus       Code = "http_status"
	CodeRateLimited      Code = "rate_limited"
	CodeInvalidResponse  Code = "invalid_response"
	CodeSteamResult      Code = "steam_result"
	CodeEntropy          Code = "entropy_failure"
)

// Redirect refusal labels, exported because a caller has to tell them apart.
//
// Only DetailRedirectDisabled says anything about the session: an endpoint that
// answers a signed request inline, redirecting at all, is Steam sending it to a
// login page. Every other label is this client's own policy declining a hop,
// and reads as a broken request rather than a rejected account.
const (
	DetailRedirectDisabled = "redirect_disabled"
	DetailRedirectLimit    = "redirect_limit"
)

// State tells callers whether an operation can be retried or needs user action.
type State string

const (
	StateInvalid   State = "invalid"
	StateDenied    State = "denied"
	StateCanceled  State = "canceled"
	StateRetryable State = "retryable"
	StateFailed    State = "failed"
)

// Error contains only values safe to display or log. It never retains a URL,
// request header, response header, body, or underlying transport error.
type Error struct {
	Code          Code
	State         State
	StatusCode    int
	EResult       int
	HasEResult    bool
	RetryAfter    time.Duration
	HasRetryAfter bool
	// Detail names which check rejected the response, so an invalid_response can
	// be diagnosed from a log line alone. It is a fixed label, never response data.
	Detail string
}

func (e *Error) Error() string {
	if e == nil {
		return "steam protocol failure"
	}
	if e.Code == CodeHTTPStatus || e.Code == CodeRateLimited {
		return "steam protocol: HTTP " + strconv.Itoa(e.StatusCode)
	}
	switch e.Code {
	case CodeInvalidRequest:
		return "steam protocol: invalid request"
	case CodeSchemeDenied:
		return "steam protocol: URL scheme denied"
	case CodeHostDenied:
		return "steam protocol: host denied"
	case CodeRedirectDenied:
		if e.Detail != "" {
			return "steam protocol: redirect denied (" + e.Detail + ")"
		}
		return "steam protocol: redirect denied"
	case CodeRequestTooLarge:
		return "steam protocol: request body too large"
	case CodeCanceled:
		return "steam protocol: request canceled"
	case CodeDeadlineExceeded:
		return "steam protocol: request deadline exceeded"
	case CodeResponseRead:
		return "steam protocol: response read failed"
	case CodeResponseTooLarge:
		return "steam protocol: response body too large"
	case CodeInvalidResponse:
		if e.Detail != "" {
			return "steam protocol: invalid response (" + e.Detail + ")"
		}
		return "steam protocol: invalid response"
	case CodeSteamResult:
		return "steam protocol: Steam result " + strconv.Itoa(e.EResult)
	case CodeEntropy:
		return "steam protocol: random source failed"
	default:
		return "steam protocol: transport failed"
	}
}

// Is preserves context cancellation checks without exposing the wrapped
// transport error that may contain a URL or credential.
func (e *Error) Is(target error) bool {
	if e == nil {
		return false
	}
	return (e.Code == CodeCanceled && target == context.Canceled) ||
		(e.Code == CodeDeadlineExceeded && target == context.DeadlineExceeded)
}

func protocolError(code Code, state State) *Error {
	return &Error{Code: code, State: state}
}
