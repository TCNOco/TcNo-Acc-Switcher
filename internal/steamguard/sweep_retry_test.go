package steamguard

import (
	"errors"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/confirmationapi"
)

// The failure the account list actually hit: a resume from sleep, DNS not up,
// every account answered "temporarily failed". Six hours is far too long to wait
// that out.
func TestRetryableSweepFailureClassification(t *testing.T) {
	for kind, want := range map[confirmationapi.FailureKind]bool{
		confirmationapi.FailureRetryable: true,
		confirmationapi.FailureTimeout:   true,
		// Coming back in thirty seconds is how a longer ban gets earned.
		confirmationapi.FailureRateLimit: false,
		// Steam answered; asking again changes nothing.
		confirmationapi.FailureReauth:  false,
		confirmationapi.FailureRefused: false,
		confirmationapi.FailureFailed:  false,
		confirmationapi.FailureInvalid: false,
		// The user turned the network off on purpose.
		confirmationapi.FailureOffline:  false,
		confirmationapi.FailureCanceled: false,
	} {
		if got := retryableSweepFailure(&confirmationapi.Error{Kind: kind}); got != want {
			t.Errorf("retryableSweepFailure(%s) = %t, want %t", kind, got, want)
		}
	}
	if retryableSweepFailure(nil) {
		t.Error("a sweep that succeeded must not be retried")
	}
	if retryableSweepFailure(errors.New("something else")) {
		t.Error("an unclassified error must not be retried")
	}
}

func TestSweepRetryBacksOffAndResetsOnSuccess(t *testing.T) {
	var r sweepRetry
	t.Cleanup(r.cancel)

	fired := make(chan struct{}, 4)
	signal := func() { fired <- struct{}{} }

	first, failures := r.note(true, signal)
	if first <= 0 || failures != 1 {
		t.Fatalf("first failure = %v/%d, want a delay and streak 1", first, failures)
	}
	second, failures := r.note(true, signal)
	if second <= first || failures != 2 {
		t.Fatalf("second failure = %v/%d, want a longer delay and streak 2", second, failures)
	}

	if delay, failures := r.note(false, signal); delay != 0 || failures != 0 {
		t.Fatalf("a reachable sweep returned %v/%d, want 0/0", delay, failures)
	}
	// The pending retry has to go with the streak, or a sweep that just worked
	// would be run again for no reason.
	r.mu.Lock()
	pending := r.timer != nil
	r.mu.Unlock()
	if pending {
		t.Fatal("a reachable sweep must cancel the retry it no longer needs")
	}
	if delay, _ := r.note(true, signal); delay != sweepRetryDelay(1) {
		t.Fatalf("delay after a reset = %v, want the first step %v", delay, sweepRetryDelay(1))
	}
}

func TestSweepRetryDelayGrowsAndCaps(t *testing.T) {
	last := time.Duration(0)
	for failures := 1; failures <= len(sweepRetryDelays); failures++ {
		got := sweepRetryDelay(failures)
		if got <= last {
			t.Fatalf("delay(%d) = %v, want more than the previous %v", failures, got, last)
		}
		last = got
	}
	if capped := sweepRetryDelay(len(sweepRetryDelays) + 50); capped != last {
		t.Fatalf("delay past the end = %v, want the cap %v", capped, last)
	}
}
