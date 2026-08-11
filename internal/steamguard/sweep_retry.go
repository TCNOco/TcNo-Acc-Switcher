package steamguard

import (
	"errors"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/confirmationapi"
)

// sweepRetryDelays is the backoff after a sweep that could not reach Steam.
//
// Both sweeps run on a cadence measured in hours, and the only other thing that
// wakes them is a vault unlock. A machine resuming from sleep sweeps before its
// network adapter is up, every account fails the same way in the same second,
// and without this the ranks, cooldowns and libraries on the account list stay
// yesterday's until one of those two things happens hours later.
var sweepRetryDelays = []time.Duration{
	30 * time.Second,
	time.Minute,
	2 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
}

// sweepRetryDelay is the wait after `failures` consecutive unreachable sweeps,
// counting the first as 1.
func sweepRetryDelay(failures int) time.Duration {
	if failures < 1 {
		failures = 1
	}
	if failures > len(sweepRetryDelays) {
		failures = len(sweepRetryDelays)
	}
	return sweepRetryDelays[failures-1]
}

// retryableSweepFailure reports whether a failed request says the network is not
// working rather than anything about the account.
//
// A rate limit is deliberately excluded: the cooldown sweep already abandons the
// rest of the list when it meets one, and coming back thirty seconds later is
// how a longer ban gets earned. A refused or reauth answer is Steam replying,
// which is not a reason to ask again either.
func retryableSweepFailure(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *confirmationapi.Error
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.Kind == confirmationapi.FailureRetryable || apiErr.Kind == confirmationapi.FailureTimeout
}

// sweepRetry re-runs a sweep that could not reach Steam.
type sweepRetry struct {
	mu       sync.Mutex
	timer    *time.Timer
	failures int
}

// note records one sweep's outcome. An unreachable sweep schedules the next one
// and returns how long away it is; a reachable one clears the streak and returns
// zero. signal is called from a timer goroutine and must not block.
func (r *sweepRetry) note(unreachable bool, signal func()) (time.Duration, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopLocked()
	if !unreachable {
		r.failures = 0
		return 0, 0
	}
	r.failures++
	delay := sweepRetryDelay(r.failures)
	r.timer = time.AfterFunc(delay, signal)
	return delay, r.failures
}

// cancel drops a scheduled retry without touching the streak. Used when a sweep
// cannot run for a reason no retry would fix - offline mode, a locked app.
func (r *sweepRetry) cancel() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopLocked()
}

func (r *sweepRetry) stopLocked() {
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
}
