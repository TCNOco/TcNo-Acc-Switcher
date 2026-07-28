package otp

import (
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu  sync.RWMutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.now
}

func (c *fakeClock) Set(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = now
}

func TestTimeStateFreshnessAndOutlier(t *testing.T) {
	local := time.Unix(1_700_000_000, 0).UTC()
	clock := &fakeClock{now: local}
	state := NewTimeStateWithLimits(clock, time.Minute, 2*time.Minute)
	if got := state.Freshness(); got != FreshnessUnavailable {
		t.Fatalf("initial freshness = %v", got)
	}
	if err := state.AcceptSample(local.Add(20*time.Second).Unix(), local); err != nil {
		t.Fatal(err)
	}
	got, status := state.Now()
	if status != FreshnessFresh || !got.Equal(local.Add(20*time.Second)) {
		t.Fatalf("corrected now = %s, %v", got, status)
	}
	if err := state.AcceptSample(local.Add(2*time.Hour).Unix(), local); err != ErrTimeSampleOutOfRange {
		t.Fatalf("outlier error = %v", err)
	}
	got, status = state.Now()
	if status != FreshnessUntrusted || !got.Equal(local) {
		t.Fatalf("outlier state = %s, %v", got, status)
	}
	if err := state.AcceptSample(local.Add(20*time.Second).Unix(), local); err != nil {
		t.Fatal(err)
	}
	clock.Set(local.Add(2*time.Minute + time.Nanosecond))
	if got := state.Freshness(); got != FreshnessStale {
		t.Fatalf("stale freshness = %v", got)
	}
	if err := state.AcceptSample(0, local); err != ErrInvalidTimeSample {
		t.Fatalf("zero sample error = %v", err)
	}
	if got := state.Freshness(); got != FreshnessUntrusted {
		t.Fatalf("invalid sample freshness = %v", got)
	}
}

func TestTimeStateClockRollbackIsStale(t *testing.T) {
	local := time.Unix(1_700_000_000, 0).UTC()
	clock := &fakeClock{now: local}
	state := NewTimeStateWithLimits(clock, time.Minute, 2*time.Minute)
	if err := state.AcceptSample(local.Add(10*time.Second).Unix(), local); err != nil {
		t.Fatal(err)
	}
	clock.Set(local.Add(-time.Second))
	got, status := state.Now()
	if status != FreshnessStale || !got.Equal(local.Add(9*time.Second)) {
		t.Fatalf("rollback state = %s, %v", got, status)
	}
}

func TestTimeStateConcurrent(t *testing.T) {
	local := time.Unix(1_700_000_000, 0)
	clock := &fakeClock{now: local}
	state := NewTimeState(clock)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				_ = state.AcceptSample(local.Add(time.Duration(i)*time.Second).Unix(), local)
				return
			}
			_, _ = state.Now()
			_ = state.Freshness()
		}(i)
	}
	wg.Wait()
}
