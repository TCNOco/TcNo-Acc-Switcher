// Package vault provides an offline encrypted store for Steam Guard account
// material. Metadata that identifies an account lives only inside the
// authenticated keyring payload.
package vault

import (
	"errors"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/securemem"
)

const (
	FormatVersion     = 1
	OuterLayerVersion = 1
	RecoveryVersion   = 1
	FixedLeaseLength  = 5 * time.Minute

	DefaultKDFMemoryKiB uint32 = 64 * 1024
	DefaultKDFPasses    uint32 = 3
	DefaultKDFLanes     uint8  = 4
)

var (
	ErrAlreadyExists           = errors.New("steam guard vault already exists")
	ErrNotFound                = errors.New("steam guard vault or record not found")
	ErrLocked                  = errors.New("steam guard vault is locked")
	ErrLeaseExpired            = errors.New("steam guard vault lease expired")
	ErrInvalidPassword         = errors.New("invalid steam guard vault password")
	ErrInvalidFormat           = errors.New("invalid steam guard vault format")
	ErrKDFBounds               = errors.New("vault KDF parameters exceed safe bounds")
	ErrHardeningUnsupported    = errors.New("filesystem access hardening is unsupported")
	ErrSecureMemory            = errors.New("secure memory refused a cached lease")
	ErrOneOperationRequired    = errors.New("secure memory is unavailable; use a one-operation unlock")
	ErrOneOperationExpired     = errors.New("one-operation unlock scope has expired")
	ErrInvalidOneOperation     = errors.New("invalid one-operation unlock callback")
	ErrOuterKeyRequired        = errors.New("Steam Guard outer encryption key is required")
	ErrInvalidOuterKey         = errors.New("invalid Steam Guard outer encryption key")
	ErrRecoveryNotConfigured   = errors.New("Steam Guard recovery wrapper is not configured")
	ErrInvalidRecoveryPassword = errors.New("invalid Steam Guard recovery password")
)

type KDFParams struct {
	Algorithm string `json:"algorithm"`
	MemoryKiB uint32 `json:"memoryKiB"`
	Passes    uint32 `json:"passes"`
	Lanes     uint8  `json:"lanes"`
	KeyBytes  uint32 `json:"keyBytes"`
}

func DefaultKDFParams() KDFParams {
	return KDFParams{
		Algorithm: "argon2id",
		MemoryKiB: DefaultKDFMemoryKiB,
		Passes:    DefaultKDFPasses,
		Lanes:     DefaultKDFLanes,
		KeyBytes:  32,
	}
}

type LeaseMode uint8

const (
	FixedLease LeaseMode = iota + 1
	ProcessLease
)

type Clock interface{ Now() time.Time }

type ClockFunc func() time.Time

func (f ClockFunc) Now() time.Time { return f() }

type Hardener interface {
	HardenDir(path string) error
	HardenFile(path string) error
}

type RecordInfo struct {
	ID        string `json:"id"`
	SteamID64 string `json:"steamId64"`
}

type Option func(*options)

type options struct {
	clock       Clock
	protector   securemem.Protector
	hardener    Hardener
	kdf         KDFParams
	recoveryKDF KDFParams
	txnHook     func(string) error
}

func WithClock(clock Clock) Option                  { return func(o *options) { o.clock = clock } }
func WithSecureMemory(p securemem.Protector) Option { return func(o *options) { o.protector = p } }
func WithHardener(h Hardener) Option                { return func(o *options) { o.hardener = h } }
func WithKDFParams(p KDFParams) Option              { return func(o *options) { o.kdf = p } }
func WithRecoveryKDFParams(p KDFParams) Option      { return func(o *options) { o.recoveryKDF = p } }

// WithTransactionHook is intended for crash-recovery testing. The hook runs at
// "after-journal" and "after-switch" and may return a simulated failure.
func WithTransactionHook(hook func(stage string) error) Option {
	return func(o *options) { o.txnHook = hook }
}
