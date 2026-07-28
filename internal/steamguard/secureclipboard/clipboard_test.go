package secureclipboard

import (
	"crypto/sha256"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakePlatform struct {
	mu            sync.Mutex
	sequence      uint32
	value         code
	hasValue      bool
	writes        int
	clears        int
	clearAttempts int
	writeErr      error
	clearFailures int
}

func (p *fakePlatform) write(value code) (writeStamp, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.writes++
	if p.writeErr != nil {
		return writeStamp{}, p.writeErr
	}
	p.sequence++
	p.value = value
	p.hasValue = true
	return writeStamp{sequence: p.sequence, digest: digestCode(value)}, nil
}

func (p *fakePlatform) clearIfUnchanged(stamp writeStamp) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clearAttempts++
	if p.clearFailures > 0 {
		p.clearFailures--
		return false, ErrUnavailable
	}
	if !p.hasValue || p.sequence != stamp.sequence || digestCode(p.value) != stamp.digest {
		return false, nil
	}
	p.sequence++
	p.value.wipe()
	p.hasValue = false
	p.clears++
	return true, nil
}

func (p *fakePlatform) replace(value string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sequence++
	p.value, _ = parseCode(value)
	p.hasValue = true
}

func (p *fakePlatform) mutateValueWithoutSequence(value string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.value, _ = parseCode(value)
	p.hasValue = true
}

func digestCode(value code) [32]byte {
	return sha256.Sum256(value[:])
}

type fakeTimer struct {
	mu      sync.Mutex
	fn      func()
	stopped bool
	fired   bool
}

func (t *fakeTimer) Stop() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopped || t.fired {
		return false
	}
	t.stopped = true
	return true
}

func (t *fakeTimer) fire() {
	t.mu.Lock()
	if t.stopped || t.fired {
		t.mu.Unlock()
		return
	}
	t.fired = true
	fn := t.fn
	t.mu.Unlock()
	fn()
}

type fakeClock struct {
	mu     sync.Mutex
	timers []*fakeTimer
}

func (c *fakeClock) AfterFunc(_ time.Duration, fn func()) timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{fn: fn}
	c.timers = append(c.timers, t)
	return t
}

func (c *fakeClock) at(index int) *fakeTimer {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.timers[index]
}

func TestCopyRejectsMalformedInputAndLifetime(t *testing.T) {
	platform := &fakePlatform{}
	manager := newManager(platform, &fakeClock{})
	for _, value := range []string{"", "ABCD", "ABCDE1", "abcde", "10OIL"} {
		if err := manager.Copy(value, time.Second); !errors.Is(err, ErrInvalidCode) {
			t.Fatalf("Copy(%q): expected invalid code, got %v", value, err)
		}
	}
	if err := manager.Copy("234BC", 0); !errors.Is(err, ErrInvalidLifetime) {
		t.Fatalf("expected invalid lifetime, got %v", err)
	}
	if err := manager.Copy("234BC", MaxClipboardLife+time.Nanosecond); !errors.Is(err, ErrInvalidLifetime) {
		t.Fatalf("expected bounded lifetime, got %v", err)
	}
	if platform.writes != 0 {
		t.Fatalf("invalid values reached platform: %d writes", platform.writes)
	}
}

func TestUnsupportedErrorIsTyped(t *testing.T) {
	err := &UnsupportedError{GOOS: "test-os"}
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported error did not unwrap: %v", err)
	}
	if got := err.Error(); got != "secure clipboard is unsupported on test-os" {
		t.Fatalf("unexpected error text: %q", got)
	}
}

func TestExpiryClearsOnlyMatchingSequenceAndValue(t *testing.T) {
	for _, test := range []struct {
		name       string
		mutate     func(*fakePlatform)
		wantClears int
	}{
		{name: "unchanged", wantClears: 1},
		{name: "new sequence", mutate: func(p *fakePlatform) { p.replace("BCDFG") }},
		{name: "new value", mutate: func(p *fakePlatform) { p.mutateValueWithoutSequence("BCDFG") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			platform := &fakePlatform{}
			clock := &fakeClock{}
			manager := newManager(platform, clock)
			if err := manager.Copy("234BC", time.Second); err != nil {
				t.Fatal(err)
			}
			if test.mutate != nil {
				test.mutate(platform)
			}
			clock.at(0).fire()
			if platform.clears != test.wantClears {
				t.Fatalf("clears = %d, want %d", platform.clears, test.wantClears)
			}
		})
	}
}

func TestReplacementCancelsPriorExpiry(t *testing.T) {
	platform := &fakePlatform{}
	clock := &fakeClock{}
	manager := newManager(platform, clock)
	if err := manager.Copy("234BC", time.Second); err != nil {
		t.Fatal(err)
	}
	if err := manager.Copy("BCDFG", time.Second); err != nil {
		t.Fatal(err)
	}
	clock.at(0).fire()
	if platform.clearAttempts != 0 {
		t.Fatalf("cancelled timer attempted clear: %d", platform.clearAttempts)
	}
	clock.at(1).fire()
	if platform.clears != 1 {
		t.Fatalf("replacement was not cleared: %d", platform.clears)
	}
}

func TestExpiryRetriesTransientPlatformFailure(t *testing.T) {
	platform := &fakePlatform{clearFailures: 1}
	clock := &fakeClock{}
	manager := newManager(platform, clock)
	if err := manager.Copy("234BC", time.Second); err != nil {
		t.Fatal(err)
	}
	clock.at(0).fire()
	if platform.clears != 0 {
		t.Fatal("failed clear unexpectedly changed clipboard")
	}
	clock.at(1).fire()
	if platform.clears != 1 || platform.clearAttempts != 2 {
		t.Fatalf("clears = %d, attempts = %d", platform.clears, platform.clearAttempts)
	}
}

func TestExpiryRetriesAreBounded(t *testing.T) {
	platform := &fakePlatform{clearFailures: maxClearAttempts + 1}
	clock := &fakeClock{}
	manager := newManager(platform, clock)
	if err := manager.Copy("234BC", time.Second); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxClearAttempts; i++ {
		clock.at(i).fire()
	}
	if platform.clearAttempts != maxClearAttempts {
		t.Fatalf("clear attempts = %d, want %d", platform.clearAttempts, maxClearAttempts)
	}
	if len(clock.timers) != maxClearAttempts {
		t.Fatalf("timers = %d, want %d", len(clock.timers), maxClearAttempts)
	}
}

func TestReplacementCancelsPendingClearRetry(t *testing.T) {
	platform := &fakePlatform{clearFailures: 1}
	clock := &fakeClock{}
	manager := newManager(platform, clock)
	if err := manager.Copy("234BC", time.Second); err != nil {
		t.Fatal(err)
	}
	clock.at(0).fire()
	if err := manager.Copy("BCDFG", time.Second); err != nil {
		t.Fatal(err)
	}
	clock.at(1).fire()
	if platform.clearAttempts != 1 {
		t.Fatalf("cancelled retry attempted clear: %d", platform.clearAttempts)
	}
	clock.at(2).fire()
	if platform.clears != 1 {
		t.Fatalf("replacement was not cleared: %d", platform.clears)
	}
}

func TestFailedReplacementKeepsPriorExpiry(t *testing.T) {
	platform := &fakePlatform{}
	clock := &fakeClock{}
	manager := newManager(platform, clock)
	if err := manager.Copy("234BC", time.Second); err != nil {
		t.Fatal(err)
	}
	platform.writeErr = ErrUnavailable
	if err := manager.Copy("BCDFG", time.Second); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected platform error, got %v", err)
	}
	platform.writeErr = nil
	clock.at(0).fire()
	if platform.clears != 1 {
		t.Fatalf("prior value was not cleared after failed replacement: %d", platform.clears)
	}
}

func TestCloseConditionallyClearsAndRejectsCopies(t *testing.T) {
	platform := &fakePlatform{}
	manager := newManager(platform, &fakeClock{})
	if err := manager.Copy("234BC", time.Second); err != nil {
		t.Fatal(err)
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	if platform.clears != 1 {
		t.Fatalf("close clears = %d", platform.clears)
	}
	if err := manager.Copy("BCDFG", time.Second); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected closed error, got %v", err)
	}
	if err := manager.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}
