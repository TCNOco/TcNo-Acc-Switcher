// Package vault provides an offline encrypted store for Steam Guard account
// material. Metadata that identifies an account lives only inside the
// authenticated keyring payload.
package vault

import (
	"errors"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/securemem"
)

// Factor types recorded in a slot. A slot lists every factor it requires.
const (
	FactorPassword     = "password"
	FactorKeyfile      = "keyfile"
	FactorRecoveryCode = "recovery"
	FactorSecurityKey  = "securitykey"
)

const (
	FormatVersion     = 2
	OuterLayerVersion = 1
	RecoveryVersion   = 1
	FixedLeaseLength  = 5 * time.Minute

	// Live-vault cost, paid on every unlock. RFC 9106's second recommended
	// profile with memory raised fourfold.
	DefaultKDFMemoryKiB uint32 = 256 * 1024
	DefaultKDFPasses    uint32 = 3
	DefaultKDFLanes     uint8  = 4

	// Backup cost. A backup is opened rarely and is the copy most likely to
	// leak, so it carries roughly ten times the live cost. Kept well under
	// maxMemoryKiB: a backup that a low-memory machine cannot open years from
	// now is a worse outcome than a backup that is merely expensive to attack,
	// and an argon2 allocation failure kills the process rather than erroring.
	BackupKDFMemoryKiB uint32 = 512 * 1024
	BackupKDFPasses    uint32 = 4
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
	ErrFactorRequired          = errors.New("Steam Guard vault requires an enrolled factor that was not supplied")
	ErrLastSlot                = errors.New("the only remaining way to open the Steam Guard vault cannot be removed")
	ErrPasswordStillInUse      = errors.New("the password was not changed: it is also used by ways in that were not supplied, and changing only the rest would leave the old password working")
	ErrSlotNotFound            = errors.New("enrolled Steam Guard factor not found")
)

// errFactorUnavailable marks a slot the supplied credentials cannot even
// attempt, as opposed to one that was attempted and refused. It never reaches
// a caller: openVaultKey turns it into ErrFactorRequired or ErrInvalidPassword.
var errFactorUnavailable = errors.New("slot factor material not supplied")

// Credentials carries the factor material offered when opening a vault. Only
// the factors a slot actually lists are consulted.
// Credentials carries the factor material offered when opening a vault. Only
// the factors a slot actually lists are consulted. SecurityKey is the value a
// hardware authenticator returned, already evaluated: the vault never talks to
// a device itself, which keeps every path here testable without one.
type Credentials struct {
	Password     string
	Keyfile      []byte
	RecoveryCode []byte
	SecurityKey  []byte
}

func PasswordOnly(password string) Credentials { return Credentials{Password: password} }

type KDFParams struct {
	Algorithm string `json:"algorithm"`
	MemoryKiB uint32 `json:"memoryKiB"`
	Passes    uint32 `json:"passes"`
	Lanes     uint8  `json:"lanes"`
	KeyBytes  uint32 `json:"keyBytes"`
}

// DefaultKDFParams is fixed, never calibrated against the current machine:
// parameters derived from local hardware produce a vault whose cost depends on
// where it happened to be created, and lanes taken from the CPU count change
// the derived key outright.
func DefaultKDFParams() KDFParams {
	return KDFParams{
		Algorithm: "argon2id",
		MemoryKiB: DefaultKDFMemoryKiB,
		Passes:    DefaultKDFPasses,
		Lanes:     DefaultKDFLanes,
		KeyBytes:  32,
	}
}

// BackupKDFParams is the cost applied to a verified backup's own header.
func BackupKDFParams() KDFParams {
	return KDFParams{
		Algorithm: "argon2id",
		MemoryKiB: BackupKDFMemoryKiB,
		Passes:    BackupKDFPasses,
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
