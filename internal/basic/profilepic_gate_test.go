package basic

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPerPlatformCoalescer_SecondClaimDuringRunQueues(t *testing.T) {
	c := newPerPlatformCoalescer()
	if !c.tryClaim("X") {
		t.Fatalf("first tryClaim should return true")
	}
	if c.tryClaim("X") {
		t.Fatalf("second tryClaim while running should return false (queued)")
	}
}

func TestPerPlatformCoalescer_DifferentPlatformsIndependent(t *testing.T) {
	c := newPerPlatformCoalescer()
	if !c.tryClaim("X") {
		t.Fatalf("X first claim should return true")
	}
	if !c.tryClaim("Y") {
		t.Fatalf("Y first claim should return true (different platform)")
	}
}

func TestPerPlatformCoalescer_OnlyOneFollowUpQueued(t *testing.T) {
	c := newPerPlatformCoalescer()
	if !c.tryClaim("X") {
		t.Fatalf("first claim should return true")
	}
	if c.tryClaim("X") {
		t.Fatalf("second claim should be queued")
	}
	if c.tryClaim("X") {
		t.Fatalf("third claim should still be coalesced (only one follow-up)")
	}
	again := c.finish("X")
	if !again {
		t.Fatalf("finish should return true (queued follow-up)")
	}
	if c.finish("X") {
		t.Fatalf("second finish after queued follow-up consumed should return false")
	}
	if !c.tryClaim("X") {
		t.Fatalf("after finish consumed the queue, platform should be free to claim again")
	}
}

func TestPerPlatformCoalescer_ConcurrentClaims(t *testing.T) {
	c := newPerPlatformCoalescer()
	const goroutines = 32
	var startCount int32
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if c.tryClaim("X") {
				atomic.AddInt32(&startCount, 1)
			}
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt32(&startCount); got != 1 {
		t.Fatalf("expected exactly 1 goroutine to win tryClaim, got %d", got)
	}
}

// --- perPlatformCooldown tests ---

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Unix(1_700_000_000, 0)}
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func TestPerPlatformCooldown_WithinWindowSkipped(t *testing.T) {
	clk := newFakeClock()
	c := newPerPlatformCooldown(30*time.Second, clk.now)
	c.markFinished("X")
	clk.advance(10 * time.Second)
	if !c.shouldSkip("X") {
		t.Fatalf("10s after finish, within 30s cooldown, should skip")
	}
}

func TestPerPlatformCooldown_AfterWindowAllowed(t *testing.T) {
	clk := newFakeClock()
	c := newPerPlatformCooldown(30*time.Second, clk.now)
	c.markFinished("X")
	clk.advance(31 * time.Second)
	if c.shouldSkip("X") {
		t.Fatalf("31s after finish, past 30s cooldown, should not skip")
	}
}

func TestPerPlatformCooldown_DifferentPlatformsIndependent(t *testing.T) {
	clk := newFakeClock()
	c := newPerPlatformCooldown(30*time.Second, clk.now)
	c.markFinished("X")
	if c.shouldSkip("Y") {
		t.Fatalf("Y has no lastRun; should not skip")
	}
}

func TestPerPlatformCooldown_BypassAllows(t *testing.T) {
	clk := newFakeClock()
	c := newPerPlatformCooldown(30*time.Second, clk.now)
	c.markFinished("X")
	if c.shouldSkipBypass("X", true) {
		t.Fatalf("bypass=true should never skip")
	}
	if !c.shouldSkipBypass("X", false) {
		t.Fatalf("bypass=false within cooldown should still skip")
	}
}

func TestPerPlatformCooldown_ResetClearsCooldown(t *testing.T) {
	clk := newFakeClock()
	c := newPerPlatformCooldown(30*time.Second, clk.now)
	c.markFinished("X")
	if !c.shouldSkip("X") {
		t.Fatalf("after markFinished, should skip")
	}
	c.reset("X")
	if c.shouldSkip("X") {
		t.Fatalf("after reset, should not skip")
	}
}
