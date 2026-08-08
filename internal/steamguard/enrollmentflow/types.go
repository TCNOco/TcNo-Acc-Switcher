// Package enrollmentflow coordinates resumable Steam authenticator enrollment.
// Secret-bearing pending state is only serialized across the encrypted-vault
// boundary and is never returned in a UI-facing result.
package enrollmentflow

import (
	"context"
	"errors"
	"runtime"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/enrollmentapi"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
)

var (
	ErrInvalidRequest                    = errors.New("invalid Steam Guard enrollment flow request")
	ErrInvalidPendingState               = errors.New("invalid Steam Guard pending enrollment state")
	ErrNoPendingEnrollment               = errors.New("Steam Guard enrollment is not pending")
	ErrAlreadyEnrolled                   = errors.New("Steam Guard is already enrolled for this account")
	ErrRevocationCodeAlreadyAcknowledged = errors.New("Steam Guard revocation code was already acknowledged")
)

const DefaultRequestTimeout = 20 * time.Second

type APIClient interface {
	AddAuthenticator(context.Context, enrollmentapi.AddRequest, time.Duration) (enrollmentapi.AddResult, error)
	FinalizeAddAuthenticator(context.Context, enrollmentapi.FinalizeRequest, time.Duration) (enrollmentapi.FinalizeResult, error)
	// QueryStatus reads the account's live two-factor state. It is used to tell
	// an authenticator Steam has already activated from one that never got that
	// far, without needing another confirmation code.
	QueryStatus(context.Context, uint64, []byte, time.Duration) (enrollmentapi.StatusResult, error)
}

// recordVault is deliberately the narrow encrypted-storage boundary used by
// the flow. New accepts only *vault.Vault; the interface exists for tests.
type recordVault interface {
	PutRecord(steamID64 string, plaintext []byte) (string, error)
	GetRecord(id string) ([]byte, error)
	ListRecords() ([]vault.RecordInfo, error)
	DeleteRecord(id string) error
}

type StartRequest struct {
	SteamID uint64
	// AccessToken authorizes the enrollment call itself. RefreshToken is not used
	// for it: it is carried through so the finished authenticator can renew its own
	// session later, which an account enrolled without one can never do.
	AccessToken       []byte
	RefreshToken      []byte
	AuthenticatorTime uint64
	// ReplaceLoginOnly permits enrolling over a session-only record, which is how
	// a login-only account is promoted to a real authenticator. It narrows the
	// refusal rather than lifting it: any other occupant is still ErrAlreadyEnrolled,
	// because enrolling over an authenticator destroys secrets held nowhere else.
	ReplaceLoginOnly bool
}

type FinalizeRequest struct {
	SteamID           uint64
	ConfirmationCode  []byte
	AuthenticatorTime uint64
}

// Status contains only non-secret state suitable for a frontend binding.
type Status struct {
	State                   enrollmentapi.State            `json:"state"`
	Confirmation            enrollmentapi.ConfirmationType `json:"confirmation"`
	PhoneHint               string                         `json:"phoneHint,omitempty"`
	RetryAfterSeconds       int64                          `json:"retryAfterSeconds,omitempty"`
	HasRetryAfter           bool                           `json:"hasRetryAfter,omitempty"`
	Pending                 bool                           `json:"pending"`
	Resumed                 bool                           `json:"resumed,omitempty"`
	RevocationViewAvailable bool                           `json:"revocationViewAvailable,omitempty"`
}

// RevocationView is the single intentional secret-bearing DTO. It is issued at
// most once per persisted enrollment. Call Destroy immediately after rendering
// or copying it; Go strings cannot guarantee in-place memory erasure.
type RevocationView struct {
	Code string `json:"code"`
}

func (v *RevocationView) Destroy() {
	if v == nil {
		return
	}
	v.Code = ""
	runtime.KeepAlive(v)
}

func (*RevocationView) String() string {
	return "Steam Guard revocation view (secret redacted)"
}

func (*RevocationView) GoString() string {
	return "Steam Guard revocation view (secret redacted)"
}

func wipe(value []byte) {
	for i := range value {
		value[i] = 0
	}
	runtime.KeepAlive(value)
}
