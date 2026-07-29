// Package enrollmentapi implements the bounded Steam authenticator enrollment
// exchange. It does not persist pending authenticator material.
package enrollmentapi

import (
	"fmt"
	"runtime"
	"time"
)

type State string

const (
	StateAwaitingSMS              State = "awaiting_sms"
	StateAwaitingEmail            State = "awaiting_email"
	StateAwaitingConfirmation     State = "awaiting_confirmation"
	StatePhoneRequired            State = "phone_required"
	StateAlreadyHasAuthenticator  State = "already_has_authenticator"
	StateRateLimited              State = "rate_limited"
	StateReauthenticationRequired State = "reauthentication_required"
	StateConfirmationCodeRejected State = "confirmation_code_rejected"
	StateAuthenticatorCodeRetry   State = "authenticator_code_retry"
	StateComplete                 State = "complete"
)

type ConfirmationType uint8

const (
	ConfirmationUnknown ConfirmationType = iota
	ConfirmationSMS
	ConfirmationEmail
)

type AddRequest struct {
	SteamID           uint64
	AccessToken       []byte
	AuthenticatorTime uint64
}

// PendingEnrollment contains the secret-bearing result that must be committed
// directly to encrypted storage before finalization begins.
type PendingEnrollment struct {
	RequestID      []byte
	SteamID        uint64
	AccessToken    []byte
	DeviceID       string
	SharedSecret   []byte
	IdentitySecret []byte
	Secret1        []byte
	RevocationCode []byte
	URI            []byte
	SerialNumber   uint64
	ServerTime     uint64
	AccountName    string
	TokenGID       string
	PhoneHint      string
	Confirmation   ConfirmationType
}

func (p *PendingEnrollment) Destroy() {
	if p == nil {
		return
	}
	wipe(p.RequestID)
	wipe(p.AccessToken)
	wipe(p.SharedSecret)
	wipe(p.IdentitySecret)
	wipe(p.Secret1)
	wipe(p.RevocationCode)
	wipe(p.URI)
	p.RequestID = nil
	p.AccessToken = nil
	p.SharedSecret = nil
	p.IdentitySecret = nil
	p.Secret1 = nil
	p.RevocationCode = nil
	p.URI = nil
	p.SteamID = 0
	p.SerialNumber = 0
	p.ServerTime = 0
	p.DeviceID = ""
	p.AccountName = ""
	p.TokenGID = ""
	p.PhoneHint = ""
	p.Confirmation = ConfirmationUnknown
}

func (*PendingEnrollment) String() string {
	return "Steam authenticator enrollment (secret material redacted)"
}
func (*PendingEnrollment) GoString() string {
	return "Steam authenticator enrollment (secret material redacted)"
}

type AddResult struct {
	State         State
	Pending       *PendingEnrollment
	RetryAfter    time.Duration
	HasRetryAfter bool
}

type FinalizeRequest struct {
	Pending           *PendingEnrollment
	RequestID         []byte
	ConfirmationCode  []byte
	AuthenticatorTime uint64
}

type FinalizeResult struct {
	State         State
	ServerTime    uint64
	RetryAfter    time.Duration
	HasRetryAfter bool
}

type SteamError struct {
	ResultCode int32
}

// Error carries Steam's result code. Without it every rejection reads the same,
// and the code is the only thing that distinguishes "wrong confirmation code"
// from "already has an authenticator" or a rate limit. It is a status number,
// not account data, so it is safe to log.
func (e *SteamError) Error() string {
	return fmt.Sprintf("%s (Steam result code %d)", ErrSteamRejected.Error(), e.ResultCode)
}

func (*SteamError) Unwrap() error { return ErrSteamRejected }

func wipe(value []byte) {
	for i := range value {
		value[i] = 0
	}
	runtime.KeepAlive(value)
}
