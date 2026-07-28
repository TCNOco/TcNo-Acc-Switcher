// Package qrattempt owns short-lived, account-bound Steam QR challenges.
// It stores challenge payloads in protected memory and performs no network
// authorization.
package qrattempt

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"TcNo-Acc-Switcher/internal/steamguard/qr"
	"TcNo-Acc-Switcher/internal/steamguard/securemem"
)

const (
	MaxTTL            = 2 * time.Minute
	MaxActiveAttempts = 64

	handleBytes              = 32
	secureMemoryBlockBytes   = 16
	maxHandleGenerationTries = 4
	maxBindingBytes          = 128
)

var (
	ErrUnavailable          = errors.New("QR attempt manager is unavailable")
	ErrInvalidConfiguration = errors.New("invalid QR attempt manager configuration")
	ErrInvalidBinding       = errors.New("invalid QR attempt binding")
	ErrInvalidChallenge     = errors.New("invalid QR attempt challenge")
	ErrInvalidTTL           = errors.New("invalid QR attempt lifetime")
	ErrInvalidHandle        = errors.New("invalid QR attempt handle")
	ErrInvalidCallback      = errors.New("invalid QR attempt callback")
	ErrNotFound             = errors.New("QR attempt not found")
	ErrBindingMismatch      = errors.New("QR attempt binding mismatch")
	ErrExpired              = errors.New("QR attempt expired")
	ErrCapacity             = errors.New("QR attempt capacity reached")
	ErrEntropy              = errors.New("QR attempt handle generation failed")
	ErrSecureMemory         = errors.New("QR attempt secure memory failure")
)

// ID is an opaque, unguessable, single-use attempt capability.
type ID string

// Binding ties an attempt to one account in one vault generation.
type Binding struct {
	AccountID       string
	VaultGeneration string
}

// Clock supplies the current time without requiring per-attempt timers.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

type attempt struct {
	binding    Binding
	expiresAt  time.Time
	payload    securemem.Handle
	payloadLen int
}

// Manager owns at most one active QR attempt per account.
type Manager struct {
	mu        sync.Mutex
	clock     Clock
	entropy   io.Reader
	protector securemem.Protector
	byID      map[ID]*attempt
	byAccount map[string]ID
}

// New returns a manager backed by crypto/rand and the platform secure-memory
// implementation.
func New() *Manager {
	return newManager(systemClock{}, rand.Reader, securemem.New())
}

// NewWithDependencies constructs a manager with injectable time, entropy, and
// secure-memory dependencies.
func NewWithDependencies(clock Clock, entropy io.Reader, protector securemem.Protector) (*Manager, error) {
	if clock == nil || entropy == nil || protector == nil {
		return nil, ErrInvalidConfiguration
	}
	return newManager(clock, entropy, protector), nil
}

func newManager(clock Clock, entropy io.Reader, protector securemem.Protector) *Manager {
	return &Manager{
		clock:     clock,
		entropy:   entropy,
		protector: protector,
		byID:      make(map[ID]*attempt),
		byAccount: make(map[string]ID),
	}
}

// Create validates, consumes, and wipes payload before storing it in protected
// memory. A successful call replaces and destroys the account's prior attempt.
func (m *Manager) Create(binding Binding, payload []byte, ttl time.Duration) (ID, error) {
	defer wipe(payload)
	if !m.available() {
		return "", ErrUnavailable
	}
	if !validBinding(binding) {
		return "", ErrInvalidBinding
	}
	if ttl <= 0 || ttl > MaxTTL {
		return "", ErrInvalidTTL
	}
	if _, err := qr.ParseChallenge(string(payload)); err != nil {
		return "", ErrInvalidChallenge
	}

	padded := make([]byte, paddedLength(len(payload)))
	copy(padded, payload)
	defer wipe(padded)
	secret, err := m.protector.Store(padded)
	if err != nil || secret == nil {
		if secret != nil {
			_ = secret.Destroy()
		}
		return "", ErrSecureMemory
	}

	m.mu.Lock()
	now := m.clock.Now()
	if _, err := m.cleanupExpiredLocked(now); err != nil {
		m.mu.Unlock()
		return "", disposeNew(secret, err)
	}

	previousID, replacing := m.byAccount[binding.AccountID]
	if !replacing && len(m.byID) >= MaxActiveAttempts {
		m.mu.Unlock()
		return "", disposeNew(secret, ErrCapacity)
	}

	id, err := m.newIDLocked()
	if err != nil {
		m.mu.Unlock()
		return "", disposeNew(secret, err)
	}
	if replacing {
		previous := m.byID[previousID]
		if previous == nil || previous.payload.Destroy() != nil {
			m.mu.Unlock()
			return "", disposeNew(secret, ErrSecureMemory)
		}
		delete(m.byID, previousID)
		delete(m.byAccount, binding.AccountID)
	}

	m.byID[id] = &attempt{
		binding:    binding,
		expiresAt:  now.Add(ttl),
		payload:    secret,
		payloadLen: len(payload),
	}
	m.byAccount[binding.AccountID] = id
	m.mu.Unlock()
	return id, nil
}

// Consume atomically removes one matching attempt before exposing its mutable
// payload to fn. The attempt is single-use even when fn fails or panics. The
// callback must not retain the supplied slice. Callback errors are returned
// unchanged; all manager-generated errors are fixed, secret-free sentinels.
func (m *Manager) Consume(id ID, binding Binding, fn func([]byte) error) error {
	if !m.available() {
		return ErrUnavailable
	}
	if !validID(id) {
		return ErrInvalidHandle
	}
	if !validBinding(binding) {
		return ErrInvalidBinding
	}
	if fn == nil {
		return ErrInvalidCallback
	}

	m.mu.Lock()
	entry, ok := m.byID[id]
	if !ok {
		m.mu.Unlock()
		return ErrNotFound
	}
	if entry.binding != binding {
		m.mu.Unlock()
		return ErrBindingMismatch
	}
	if !m.clock.Now().Before(entry.expiresAt) {
		if entry.payload.Destroy() != nil {
			m.mu.Unlock()
			return ErrSecureMemory
		}
		m.removeLocked(id, entry)
		m.mu.Unlock()
		return ErrExpired
	}
	m.removeLocked(id, entry)
	m.mu.Unlock()

	return consumeAndDestroy(entry, fn)
}

// Inspect exposes a short-lived mutable copy to fn without consuming the
// attempt. It is intended for fetching requestor details before an explicit
// approval. The callback must not retain the supplied slice.
func (m *Manager) Inspect(id ID, binding Binding, fn func([]byte) error) error {
	if !m.available() {
		return ErrUnavailable
	}
	if !validID(id) {
		return ErrInvalidHandle
	}
	if !validBinding(binding) {
		return ErrInvalidBinding
	}
	if fn == nil {
		return ErrInvalidCallback
	}

	m.mu.Lock()
	entry, ok := m.byID[id]
	if !ok {
		m.mu.Unlock()
		return ErrNotFound
	}
	if entry.binding != binding {
		m.mu.Unlock()
		return ErrBindingMismatch
	}
	if !m.clock.Now().Before(entry.expiresAt) {
		if entry.payload.Destroy() != nil {
			m.mu.Unlock()
			return ErrSecureMemory
		}
		m.removeLocked(id, entry)
		m.mu.Unlock()
		return ErrExpired
	}
	payload, err := copyPayload(entry)
	m.mu.Unlock()
	if err != nil {
		return err
	}
	defer wipe(payload)
	return fn(payload)
}

// RevokeAccount destroys the active attempt for accountID, if one exists.
func (m *Manager) RevokeAccount(accountID string) error {
	if !m.available() {
		return ErrUnavailable
	}
	if !validBindingPart(accountID) {
		return ErrInvalidBinding
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byAccount[accountID]
	if !ok {
		return nil
	}
	entry := m.byID[id]
	if entry == nil || entry.payload.Destroy() != nil {
		return ErrSecureMemory
	}
	m.removeLocked(id, entry)
	return nil
}

// RevokeAll destroys every active attempt. Entries whose secure-memory handle
// cannot be destroyed remain tracked so a later call can retry.
func (m *Manager) RevokeAll() error {
	if !m.available() {
		return ErrUnavailable
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	failed := false
	for id, entry := range m.byID {
		if entry.payload.Destroy() != nil {
			failed = true
			continue
		}
		m.removeLocked(id, entry)
	}
	if failed {
		return ErrSecureMemory
	}
	return nil
}

// CleanupExpired synchronously destroys attempts at or beyond their expiry.
func (m *Manager) CleanupExpired() (int, error) {
	if !m.available() {
		return 0, ErrUnavailable
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cleanupExpiredLocked(m.clock.Now())
}

func (m *Manager) cleanupExpiredLocked(now time.Time) (int, error) {
	removed := 0
	failed := false
	for id, entry := range m.byID {
		if now.Before(entry.expiresAt) {
			continue
		}
		if entry.payload.Destroy() != nil {
			failed = true
			continue
		}
		m.removeLocked(id, entry)
		removed++
	}
	if failed {
		return removed, ErrSecureMemory
	}
	return removed, nil
}

func (m *Manager) available() bool {
	return m != nil && m.clock != nil && m.entropy != nil && m.protector != nil && m.byID != nil && m.byAccount != nil
}

func (m *Manager) removeLocked(id ID, entry *attempt) {
	delete(m.byID, id)
	if active, ok := m.byAccount[entry.binding.AccountID]; ok && active == id {
		delete(m.byAccount, entry.binding.AccountID)
	}
}

func (m *Manager) newIDLocked() (ID, error) {
	var raw [handleBytes]byte
	for range maxHandleGenerationTries {
		if _, err := io.ReadFull(m.entropy, raw[:]); err != nil {
			wipe(raw[:])
			return "", ErrEntropy
		}
		id := ID(base64.RawURLEncoding.EncodeToString(raw[:]))
		wipe(raw[:])
		if _, exists := m.byID[id]; !exists {
			return id, nil
		}
	}
	return "", ErrEntropy
}

func consumeAndDestroy(entry *attempt, fn func([]byte) error) (result error) {
	defer func() {
		if entry.payload.Destroy() != nil {
			result = ErrSecureMemory
		}
	}()
	var callbackErr error
	withErr := entry.payload.With(func(protectedCopy []byte) error {
		defer wipe(protectedCopy)
		if entry.payloadLen <= 0 || len(protectedCopy) < entry.payloadLen {
			return ErrSecureMemory
		}
		callbackErr = fn(protectedCopy[:entry.payloadLen])
		return callbackErr
	})
	if callbackErr != nil {
		return callbackErr
	}
	if withErr != nil {
		return ErrSecureMemory
	}
	return nil
}

func copyPayload(entry *attempt) ([]byte, error) {
	var payload []byte
	withErr := entry.payload.With(func(protectedCopy []byte) error {
		defer wipe(protectedCopy)
		if entry.payloadLen <= 0 || len(protectedCopy) < entry.payloadLen {
			return ErrSecureMemory
		}
		payload = append([]byte(nil), protectedCopy[:entry.payloadLen]...)
		return nil
	})
	if withErr != nil || len(payload) != entry.payloadLen {
		wipe(payload)
		return nil, ErrSecureMemory
	}
	return payload, nil
}

func disposeNew(secret securemem.Handle, cause error) error {
	if secret.Destroy() != nil {
		return ErrSecureMemory
	}
	return cause
}

func validBinding(binding Binding) bool {
	return validBindingPart(binding.AccountID) && validBindingPart(binding.VaultGeneration)
}

func validBindingPart(value string) bool {
	if len(value) == 0 || len(value) > maxBindingBytes || !utf8.ValidString(value) || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validID(id ID) bool {
	if len(id) != base64.RawURLEncoding.EncodedLen(handleBytes) {
		return false
	}
	var decoded [handleBytes]byte
	count, err := base64.RawURLEncoding.Decode(decoded[:], []byte(id))
	wipe(decoded[:])
	return err == nil && count == handleBytes
}

func paddedLength(length int) int {
	return ((length + secureMemoryBlockBytes - 1) / secureMemoryBlockBytes) * secureMemoryBlockBytes
}

func wipe(value []byte) {
	clear(value)
	runtime.KeepAlive(value)
}
