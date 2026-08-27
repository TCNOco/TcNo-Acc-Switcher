package otp

import (
	"errors"
	"sync"
	"time"
)

const (
	// DefaultMaxOffset limits accepted server-time corrections.
	DefaultMaxOffset = 24 * time.Hour
	// DefaultFreshness is the period a server-time sample remains usable.
	DefaultFreshness = 10 * time.Minute
)

var (
	// ErrInvalidTimeSample reports a sample with an invalid timestamp.
	ErrInvalidTimeSample = errors.New("invalid Steam Guard time sample")
	// ErrTimeSampleOutOfRange reports a sample whose correction exceeds the configured bound.
	ErrTimeSampleOutOfRange = errors.New("Steam Guard time sample offset exceeds limit")
)

// Clock supplies local time.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Freshness reports whether the current correction is absent, fresh, stale, or untrusted.
type Freshness uint8

const (
	FreshnessUnavailable Freshness = iota
	FreshnessFresh
	FreshnessStale
	FreshnessUntrusted
)

// TimeState stores a bounded server-time correction. Its methods are safe for concurrent use.
type TimeState struct {
	mu        sync.RWMutex
	clock     Clock
	maxOffset time.Duration
	maxAge    time.Duration
	offset    time.Duration
	sampledAt time.Time
	hasSample bool
	untrusted bool
}

// NewTimeState constructs a state using default offset and freshness limits.
func NewTimeState(clock Clock) *TimeState {
	return NewTimeStateWithLimits(clock, DefaultMaxOffset, DefaultFreshness)
}

// NewTimeStateWithLimits constructs a state with explicit positive limits.
// Non-positive limits use the defaults.
func NewTimeStateWithLimits(clock Clock, maxOffset, maxAge time.Duration) *TimeState {
	if clock == nil {
		clock = systemClock{}
	}
	if maxOffset <= 0 {
		maxOffset = DefaultMaxOffset
	}
	if maxAge <= 0 {
		maxAge = DefaultFreshness
	}
	return &TimeState{clock: clock, maxOffset: maxOffset, maxAge: maxAge}
}

// AcceptSample records a trusted server Unix timestamp observed at sampledAt.
// Invalid samples discard the prior correction and mark the state untrusted.
func (s *TimeState) AcceptSample(serverUnix int64, sampledAt time.Time) error {
	if serverUnix <= 0 || sampledAt.IsZero() {
		s.MarkUntrusted()
		return ErrInvalidTimeSample
	}
	serverTime := time.Unix(serverUnix, 0)
	offset := serverTime.Sub(sampledAt)

	s.mu.Lock()
	defer s.mu.Unlock()
	if offset > s.maxOffset || offset < -s.maxOffset {
		s.offset = 0
		s.sampledAt = time.Time{}
		s.hasSample = false
		s.untrusted = true
		return ErrTimeSampleOutOfRange
	}
	s.offset = offset
	s.sampledAt = sampledAt
	s.hasSample = true
	s.untrusted = false
	return nil
}

// MarkUntrusted discards the correction after a response fails validation.
func (s *TimeState) MarkUntrusted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.offset = 0
	s.sampledAt = time.Time{}
	s.hasSample = false
	s.untrusted = true
}

// Now returns the corrected local time and the sample's freshness.
func (s *TimeState) Now() (time.Time, Freshness) {
	now := s.clock.Now()
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.untrusted {
		return now, FreshnessUntrusted
	}
	if !s.hasSample {
		return now, FreshnessUnavailable
	}
	status := FreshnessFresh
	if age := now.Sub(s.sampledAt); age < 0 || age > s.maxAge {
		status = FreshnessStale
	}
	offset := s.offset
	if offset > s.maxOffset {
		offset = s.maxOffset
	} else if offset < -s.maxOffset {
		offset = -s.maxOffset
	}
	return now.Add(offset), status
}

// Freshness returns the current age status of the accepted sample.
func (s *TimeState) Freshness() Freshness {
	_, status := s.Now()
	return status
}
