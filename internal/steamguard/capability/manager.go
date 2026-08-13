// Package capability issues short-lived, window-bound Steam Guard capabilities.
package capability

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"sync"
)

const tokenBytes = 32

var ErrInvalidCapability = errors.New("invalid Steam Guard window capability")

type Manager struct {
	mu     sync.RWMutex
	grants map[[sha256.Size]byte]Binding
	random io.Reader
}

// Binding identifies the exact frontend authority a token was issued for.
type Binding struct {
	WindowName      string
	AccountID       string
	Scope           string
	LeaseID         string
	VaultGeneration string
}

func NewManager() *Manager {
	return &Manager{grants: make(map[[sha256.Size]byte]Binding), random: rand.Reader}
}

func (m *Manager) Issue(binding Binding) (string, error) {
	binding = normalizeBinding(binding)
	if !validBinding(binding) {
		return "", ErrInvalidCapability
	}
	raw := make([]byte, tokenBytes)
	if _, err := io.ReadFull(m.random, raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	digest := tokenDigest(token)
	m.mu.Lock()
	if m.grants == nil {
		m.grants = make(map[[sha256.Size]byte]Binding)
	}
	m.grants[digest] = binding
	m.mu.Unlock()
	wipe(raw)
	return token, nil
}

func (m *Manager) Validate(binding Binding, token string) error {
	binding = normalizeBinding(binding)
	if !validBinding(binding) || token == "" {
		return ErrInvalidCapability
	}
	digest := tokenDigest(token)
	m.mu.RLock()
	want, ok := m.grants[digest]
	m.mu.RUnlock()
	if !ok || !sameBinding(want, binding) {
		return ErrInvalidCapability
	}
	return nil
}

func (m *Manager) Resolve(token string) (Binding, error) {
	if token == "" {
		return Binding{}, ErrInvalidCapability
	}
	digest := tokenDigest(token)
	m.mu.RLock()
	binding, ok := m.grants[digest]
	m.mu.RUnlock()
	if !ok {
		return Binding{}, ErrInvalidCapability
	}
	return binding, nil
}

func (m *Manager) Revoke(token string) {
	digest := tokenDigest(token)
	m.mu.Lock()
	delete(m.grants, digest)
	m.mu.Unlock()
}

// Rebind moves every live grant onto a new vault generation, leaving the issued
// tokens themselves valid.
//
// Only for a write that changes no authority - a background renewal of stored
// Steam session tokens. It does not alter who may read what, so the windows
// holding a capability across it must survive. A write that re-keys the vault or
// changes which records exist still has to orphan the capabilities bound to the
// generation it replaced, and must not call this.
func (m *Manager) Rebind(generation string) {
	generation = strings.TrimSpace(generation)
	if generation == "" {
		return
	}
	m.mu.Lock()
	for digest, binding := range m.grants {
		binding.VaultGeneration = generation
		m.grants[digest] = binding
	}
	m.mu.Unlock()
}

func (m *Manager) RevokeWindow(windowName string) {
	windowName = strings.TrimSpace(windowName)
	m.mu.Lock()
	for digest, binding := range m.grants {
		if binding.WindowName == windowName {
			delete(m.grants, digest)
		}
	}
	m.mu.Unlock()
}

func (m *Manager) RevokeAll() {
	m.mu.Lock()
	clear(m.grants)
	m.mu.Unlock()
}

func tokenDigest(token string) [sha256.Size]byte {
	return sha256.Sum256([]byte("tcno-steamguard-window-v2\x00" + token))
}

func normalizeBinding(binding Binding) Binding {
	binding.WindowName = strings.TrimSpace(binding.WindowName)
	binding.AccountID = strings.TrimSpace(binding.AccountID)
	binding.Scope = strings.TrimSpace(binding.Scope)
	binding.LeaseID = strings.TrimSpace(binding.LeaseID)
	binding.VaultGeneration = strings.TrimSpace(binding.VaultGeneration)
	return binding
}

func validBinding(binding Binding) bool {
	return binding.WindowName != "" && binding.AccountID != "" && binding.Scope != "" && binding.LeaseID != ""
}

func sameBinding(left, right Binding) bool {
	leftWindow := []byte(left.WindowName)
	rightWindow := []byte(right.WindowName)
	leftAccount := []byte(left.AccountID)
	rightAccount := []byte(right.AccountID)
	leftScope := []byte(left.Scope)
	rightScope := []byte(right.Scope)
	leftLease := []byte(left.LeaseID)
	rightLease := []byte(right.LeaseID)
	leftGeneration := []byte(left.VaultGeneration)
	rightGeneration := []byte(right.VaultGeneration)
	return subtle.ConstantTimeCompare(leftWindow, rightWindow) == 1 &&
		subtle.ConstantTimeCompare(leftAccount, rightAccount) == 1 &&
		subtle.ConstantTimeCompare(leftScope, rightScope) == 1 &&
		subtle.ConstantTimeCompare(leftLease, rightLease) == 1 &&
		subtle.ConstantTimeCompare(leftGeneration, rightGeneration) == 1
}

func wipe(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
