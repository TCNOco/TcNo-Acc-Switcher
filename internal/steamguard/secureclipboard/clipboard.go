// Package secureclipboard copies short-lived Steam Guard codes through the
// native clipboard while preventing an expiry timer from erasing newer data.
package secureclipboard

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"
)

const (
	codeLength       = 5
	steamAlphabet    = "23456789BCDFGHJKMNPQRTVWXY"
	MaxClipboardLife = 31 * time.Second
	clearRetryDelay  = 100 * time.Millisecond
	maxClearAttempts = 4
)

var (
	ErrInvalidCode     = errors.New("invalid Steam Guard code")
	ErrInvalidLifetime = errors.New("invalid clipboard lifetime")
	ErrUnsupported     = errors.New("secure clipboard is unsupported")
	ErrUnavailable     = errors.New("secure clipboard is unavailable")
	ErrClosed          = errors.New("secure clipboard is closed")
)

// UnsupportedError identifies a platform on which the secure clipboard is
// deliberately unavailable. It unwraps to ErrUnsupported.
type UnsupportedError struct {
	GOOS string
}

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("secure clipboard is unsupported on %s", e.GOOS)
}

func (e *UnsupportedError) Unwrap() error { return ErrUnsupported }

type code [codeLength]byte

func parseCode(value string) (code, error) {
	var parsed code
	if len(value) != codeLength {
		return parsed, ErrInvalidCode
	}
	for i := range parsed {
		if !isSteamCodeByte(value[i]) {
			parsed.wipe()
			return parsed, ErrInvalidCode
		}
		parsed[i] = value[i]
	}
	return parsed, nil
}

func isSteamCodeByte(value byte) bool {
	for i := 0; i < len(steamAlphabet); i++ {
		if steamAlphabet[i] == value {
			return true
		}
	}
	return false
}

func (c *code) wipe() {
	for i := range c {
		c[i] = 0
	}
	runtime.KeepAlive(c)
}

type writeStamp struct {
	sequence uint32
	digest   [sha256.Size]byte
}

type clipboardPlatform interface {
	write(code) (writeStamp, error)
	clearIfUnchanged(writeStamp) (bool, error)
}

type timer interface {
	Stop() bool
}

type clock interface {
	AfterFunc(time.Duration, func()) timer
}

type systemClock struct{}

func (systemClock) AfterFunc(delay time.Duration, fn func()) timer {
	return time.AfterFunc(delay, fn)
}

// Manager owns at most one pending conditional clipboard clear.
type Manager struct {
	mu         sync.Mutex
	platform   clipboardPlatform
	clock      clock
	timer      timer
	active     *writeStamp
	generation uint64
	closed     bool
}

// New creates a secure clipboard manager for the current platform.
func New() *Manager {
	return newManager(newPlatform(), systemClock{})
}

func newManager(platform clipboardPlatform, clock clock) *Manager {
	return &Manager{platform: platform, clock: clock}
}

// Copy writes an exact five-character Steam Guard code and conditionally
// clears it after lifetime. The lifetime is bounded to the Steam code window.
func (m *Manager) Copy(value string, lifetime time.Duration) error {
	parsed, err := parseCode(value)
	if err != nil {
		return err
	}
	defer parsed.wipe()
	if lifetime <= 0 || lifetime > MaxClipboardLife {
		return ErrInvalidLifetime
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}

	stamp, err := m.platform.write(parsed)
	if err != nil {
		return err
	}
	if m.timer != nil {
		m.timer.Stop()
	}
	m.generation++
	generation := m.generation
	m.active = &stamp
	m.timer = m.clock.AfterFunc(lifetime, func() {
		m.expire(generation, stamp, 1)
	})
	return nil
}

// Clear clears the managed clipboard value only when both its sequence number
// and content digest still match the last successful Copy.
func (m *Manager) Clear() (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.clearLocked()
}

func (m *Manager) clearLocked() (bool, error) {
	if m.timer != nil {
		m.timer.Stop()
		m.timer = nil
	}
	m.generation++
	if m.active == nil {
		return false, nil
	}
	stamp := *m.active
	cleared, err := m.platform.clearIfUnchanged(stamp)
	if err == nil {
		m.active = nil
	}
	return cleared, err
}

func (m *Manager) expire(generation uint64, stamp writeStamp, attempt int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || generation != m.generation || m.active == nil || *m.active != stamp {
		return
	}
	_, err := m.platform.clearIfUnchanged(stamp)
	if err != nil && attempt < maxClearAttempts {
		m.timer = m.clock.AfterFunc(clearRetryDelay, func() {
			m.expire(generation, stamp, attempt+1)
		})
		return
	}
	m.timer = nil
	m.active = nil
}

// Close cancels the timer, conditionally clears the current managed value, and
// prevents further copies. It is safe to call more than once.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	_, err := m.clearLocked()
	m.closed = true
	return err
}
