package basic

import (
	"sync"
	"time"
)

// perPlatformCoalescer ensures at most one background refresh runs per platform,
// and at most one follow-up is queued when additional requests arrive during a run.
type perPlatformCoalescer struct {
	mu      sync.Mutex
	running map[string]bool
	queued  map[string]bool
}

func newPerPlatformCoalescer() *perPlatformCoalescer {
	return &perPlatformCoalescer{
		running: map[string]bool{},
		queued:  map[string]bool{},
	}
}

// tryClaim returns true if the caller should run a refresh right now.
// If a refresh is already running for this platform, the call is recorded as
// a queued follow-up and false is returned.
func (c *perPlatformCoalescer) tryClaim(platformKey string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running[platformKey] {
		c.queued[platformKey] = true
		return false
	}
	c.running[platformKey] = true
	return true
}

// finish clears the running flag for platformKey and returns true if a
// follow-up run was queued.
func (c *perPlatformCoalescer) finish(platformKey string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	again := c.queued[platformKey]
	c.queued[platformKey] = false
	c.running[platformKey] = false
	return again
}

// perPlatformCooldown gates refresh runs so the same platform cannot be
// re-scanned more frequently than the configured interval. State is
// in-memory only; it does not survive process restarts.
type perPlatformCooldown struct {
	cooldown time.Duration
	clock    func() time.Time

	mu      sync.Mutex
	lastRun map[string]time.Time
}

func newPerPlatformCooldown(cooldown time.Duration, clock func() time.Time) *perPlatformCooldown {
	return &perPlatformCooldown{
		cooldown: cooldown,
		clock:    clock,
		lastRun:  map[string]time.Time{},
	}
}

// shouldSkip reports whether a refresh for platformKey is still inside the cooldown window.
func (c *perPlatformCooldown) shouldSkip(platformKey string) bool {
	return c.shouldSkipBypass(platformKey, false)
}

// shouldSkipBypass honours bypass to allow explicit user-initiated refreshes
// (e.g. "Refresh all images" buttons) to ignore the cooldown.
func (c *perPlatformCooldown) shouldSkipBypass(platformKey string, bypass bool) bool {
	if bypass {
		return false
	}
	c.mu.Lock()
	last, ok := c.lastRun[platformKey]
	c.mu.Unlock()
	if !ok {
		return false
	}
	return c.clock().Sub(last) < c.cooldown
}

// markFinished records that a refresh for platformKey just completed.
func (c *perPlatformCooldown) markFinished(platformKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastRun[platformKey] = c.clock()
}

// reset clears the cooldown for platformKey so the next request is not throttled.
func (c *perPlatformCooldown) reset(platformKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.lastRun, platformKey)
}
