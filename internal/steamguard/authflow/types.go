// Package authflow manages short-lived Steam credential authentication
// sessions without exposing Steam request identifiers or issued tokens to UI
// bindings.
package authflow

import (
	"context"
	"errors"
	"io"
	"strconv"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/protocol"
)

// Steam EResult values worth naming: each one means the same attempt cannot
// succeed on retry, which is the difference the user needs to see.
const (
	eresultInvalidPassword            = 5
	eresultExpired                    = 27
	eresultRateLimitExceeded          = 84
	eresultAccountLoginDeniedThrottle = 87
	eresultTwoFactorCodeMismatch      = 88
)

const (
	DefaultCapacity          = 8
	DefaultSessionTTL        = 5 * time.Minute
	DefaultOperationTimeout  = 20 * time.Second
	DefaultSweepInterval     = time.Second
	DefaultTombstoneTTL      = 5 * time.Minute
	DefaultTombstoneCapacity = 64
)

// Clock is the small time boundary needed for deterministic expiry tests.
type Clock interface {
	Now() time.Time
	After(time.Duration) <-chan time.Time
}

// Config bounds live sessions, remote calls, and replay tombstones. Zero
// values select conservative defaults.
type Config struct {
	Capacity          int
	SessionTTL        time.Duration
	OperationTimeout  time.Duration
	SweepInterval     time.Duration
	TombstoneTTL      time.Duration
	TombstoneCapacity int
	Entropy           io.Reader
	Clock             Clock
}

// Client is the only Steam protocol surface used by Manager. Implementations
// borrow password and code only for the duration of their respective calls.
type Client interface {
	Begin(context.Context, protocol.PasswordCredentialsRequest, []byte, time.Duration) (protocol.BeginCredentialsResult, error)
	SubmitCode(context.Context, protocol.AuthSession, protocol.ChallengeType, []byte, time.Duration) (protocol.ChallengeResult, error)
	Poll(context.Context, protocol.AuthSession, time.Duration) (protocol.PollResult, error)
}

// Binding ties a session to its account, encrypted-vault generation, and the
// caller's capability scope. It must be supplied unchanged for every action.
type Binding struct {
	AccountID       string
	ExpectedSteamID uint64
	VaultGeneration string
	CapabilityID    string
}

func (Binding) String() string   { return "Steam authentication binding [redacted]" }
func (Binding) GoString() string { return "authflow.Binding{[redacted]}" }

// State is the non-secret next action suitable for a frontend binding.
type State string

const (
	StateWaiting           State = "waiting"
	StateChallengeRequired State = "challenge_required"
	StateAgreementRequired State = "agreement_required"
	StateCodeAccepted      State = "code_accepted"
	StateAuthorizedReady   State = "authorized_ready"
)

// Challenge is a safe projection of Steam's allowed confirmation methods.
type Challenge string

const (
	ChallengeNone               Challenge = "none"
	ChallengeEmailCode          Challenge = "email_code"
	ChallengeDeviceCode         Challenge = "device_code"
	ChallengeDeviceConfirmation Challenge = "device_confirmation"
	ChallengeEmailConfirmation  Challenge = "email_confirmation"
	ChallengeUnsupported        Challenge = "unsupported"
)

// Status intentionally excludes Steam request IDs, challenge answers, account
// names, server messages, agreement URLs, and issued credentials.
type Status struct {
	Handle              string      `json:"handle"`
	State               State       `json:"state"`
	Challenges          []Challenge `json:"challenges,omitempty"`
	CanSubmitEmailCode  bool        `json:"canSubmitEmailCode"`
	CanSubmitDeviceCode bool        `json:"canSubmitDeviceCode"`
	CanPoll             bool        `json:"canPoll"`
	PollAfterMillis     int64       `json:"pollAfterMillis,omitempty"`
	ExpiresAtUnix       int64       `json:"expiresAtUnix"`
}

// Consumer is invoked at most once with borrowed credential slices. The
// manager wipes the slices immediately after the callback returns or panics.
// Callers must copy them directly into protected storage and must not retain
// references to them.
type Consumer func(steamID uint64, accountName, accessToken, refreshToken, guardData []byte, hadRemoteInteraction bool) error

// ErrorKind is a stable, secret-free failure classification.
type ErrorKind string

const (
	ErrorInvalid         ErrorKind = "invalid"
	ErrorConflict        ErrorKind = "conflict"
	ErrorCapacity        ErrorKind = "capacity"
	ErrorNotFound        ErrorKind = "not_found"
	ErrorGone            ErrorKind = "gone"
	ErrorBindingMismatch ErrorKind = "binding_mismatch"
	ErrorBusy            ErrorKind = "busy"
	ErrorTooSoon         ErrorKind = "too_soon"
	ErrorNotAuthorized   ErrorKind = "not_authorized"
	ErrorCanceled        ErrorKind = "canceled"
	ErrorTimeout         ErrorKind = "timeout"
	ErrorRateLimited     ErrorKind = "rate_limited"
	ErrorProtocol        ErrorKind = "protocol"
	ErrorConsumer        ErrorKind = "consumer_failed"
	ErrorClosed          ErrorKind = "closed"
)

// Error never wraps a transport or protocol error, because those values may
// have been supplied by an untrusted implementation of Client.
type Error struct {
	Kind          ErrorKind
	RetryAfter    time.Duration
	HasRetryAfter bool
	// Cause detail for ErrorProtocol. These name why Steam refused, so the user
	// is told what went wrong instead of retrying an attempt that cannot succeed.
	ProtocolCode string
	StatusCode   int
	EResult      int
	HasEResult   bool
}

func (e *Error) Error() string {
	if e == nil {
		return "Steam authentication failed"
	}
	switch e.Kind {
	case ErrorInvalid:
		return "Steam authentication request is invalid"
	case ErrorConflict:
		return "Steam authentication is already active for this account"
	case ErrorCapacity:
		return "Steam authentication session capacity was reached"
	case ErrorNotFound:
		return "Steam authentication session was not found"
	case ErrorGone:
		return "Steam authentication session is no longer available"
	case ErrorBindingMismatch:
		return "Steam authentication session binding does not match"
	case ErrorBusy:
		return "Steam authentication session is busy"
	case ErrorTooSoon:
		return "Steam authentication polling is temporarily delayed"
	case ErrorNotAuthorized:
		return "Steam authentication credentials are not ready"
	case ErrorCanceled:
		return "Steam authentication was canceled"
	case ErrorTimeout:
		return "Steam authentication timed out"
	case ErrorRateLimited:
		return "Steam authentication is rate limited"
	case ErrorProtocol:
		return "Steam authentication failed: " + e.protocolCause()
	case ErrorConsumer:
		return "Steam authentication credential transfer failed"
	case ErrorClosed:
		return "Steam authentication manager is closed"
	default:
		return "Steam authentication protocol failed"
	}
}

// protocolCause names the refusal in words the user can act on. Steam reports the
// common dead ends (wrong password, expired session) as an EResult on an HTTP 200,
// so an unnamed cause would read as a transient failure worth retrying.
func (e *Error) protocolCause() string {
	if e.HasEResult {
		switch e.EResult {
		case eresultInvalidPassword:
			return "Steam rejected the account name or password"
		case eresultAccountLoginDeniedThrottle:
			return "Steam is throttling sign-in attempts for this account; wait before trying again"
		case eresultRateLimitExceeded:
			return "Steam rate limit reached; wait before trying again"
		case eresultTwoFactorCodeMismatch:
			return "Steam rejected the Steam Guard code"
		case eresultExpired:
			return "the sign-in attempt expired"
		}
		return "Steam refused the request (result " + strconv.Itoa(e.EResult) + ")"
	}
	if e.StatusCode != 0 {
		return "Steam returned HTTP " + strconv.Itoa(e.StatusCode)
	}
	if e.ProtocolCode != "" {
		return e.ProtocolCode
	}
	return "the Steam protocol call failed"
}

func (e *Error) Is(target error) bool {
	return e != nil && ((e.Kind == ErrorCanceled && target == context.Canceled) ||
		(e.Kind == ErrorTimeout && target == context.DeadlineExceeded))
}

func flowError(kind ErrorKind) *Error { return &Error{Kind: kind} }

func classifyClientError(err error) *Error {
	if errors.Is(err, context.Canceled) {
		return flowError(ErrorCanceled)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return flowError(ErrorTimeout)
	}
	var protocolErr *protocol.Error
	if errors.As(err, &protocolErr) {
		if protocolErr.Code == protocol.CodeRateLimited {
			return &Error{Kind: ErrorRateLimited, RetryAfter: protocolErr.RetryAfter, HasRetryAfter: protocolErr.HasRetryAfter}
		}
		if protocolErr.Code == protocol.CodeCanceled {
			return flowError(ErrorCanceled)
		}
		if protocolErr.Code == protocol.CodeDeadlineExceeded {
			return flowError(ErrorTimeout)
		}
		code := string(protocolErr.Code)
		if protocolErr.Detail != "" {
			code += ": " + protocolErr.Detail
		}
		return &Error{
			Kind:         ErrorProtocol,
			ProtocolCode: code,
			StatusCode:   protocolErr.StatusCode,
			EResult:      protocolErr.EResult,
			HasEResult:   protocolErr.HasEResult,
		}
	}
	return flowError(ErrorProtocol)
}
