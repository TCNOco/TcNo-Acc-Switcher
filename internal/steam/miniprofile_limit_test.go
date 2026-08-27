package steam

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fakeClock lets the window be driven without waiting out real minutes.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newTestWindow(capacity int, window time.Duration) (*requestWindow, *fakeClock) {
	clock := &fakeClock{now: time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)}
	w := newRequestWindow(capacity, window)
	w.now = clock.Now
	return w, clock
}

// The whole point of a sliding window rather than a fixed gap: a burst that fits
// is admitted at once. A dozen accounts opening a page must not be paced apart.
func TestWindowAdmitsAWholeBurstAtOnce(t *testing.T) {
	w, _ := newTestWindow(15, time.Minute)
	for i := 0; i < 15; i++ {
		if wait, ok := w.reserve(); !ok {
			t.Fatalf("request %d was deferred by %s; the whole burst should fit", i+1, wait)
		}
	}
}

func TestWindowDefersTheRequestPastCapacity(t *testing.T) {
	w, _ := newTestWindow(3, time.Minute)
	for i := 0; i < 3; i++ {
		if _, ok := w.reserve(); !ok {
			t.Fatalf("request %d should have been admitted", i+1)
		}
	}
	wait, ok := w.reserve()
	if ok {
		t.Fatal("a fourth request was admitted into a window of three")
	}
	if wait != time.Minute {
		t.Fatalf("wait = %s, want the full window since all three landed together", wait)
	}
}

// A window that never released its oldest slot would stall the refresh for good.
func TestWindowReleasesSlotsAsTheyAgeOut(t *testing.T) {
	w, clock := newTestWindow(2, time.Minute)
	w.reserve()
	clock.advance(30 * time.Second)
	w.reserve()

	if wait, ok := w.reserve(); ok {
		t.Fatal("a third request was admitted into a window of two")
	} else if wait != 30*time.Second {
		t.Fatalf("wait = %s, want 30s until the first slot ages out", wait)
	}

	clock.advance(31 * time.Second)
	if _, ok := w.reserve(); !ok {
		t.Fatal("the oldest slot aged out but no request was admitted")
	}
}

// Steam answers a refusal with a bare 500 and no Retry-After, so the stand-down
// is ours to enforce. Measured at fifty to sixty seconds.
func TestPenaltyBlocksEvenAnOtherwiseFreeWindow(t *testing.T) {
	w, clock := newTestWindow(15, time.Minute)
	w.penalise(65 * time.Second)

	wait, ok := w.reserve()
	if ok {
		t.Fatal("a request was admitted while the endpoint was standing down")
	}
	if wait != 65*time.Second {
		t.Fatalf("wait = %s, want the full penalty", wait)
	}

	clock.advance(66 * time.Second)
	if _, ok := w.reserve(); !ok {
		t.Fatal("the penalty lapsed but no request was admitted")
	}
}

// Two accounts finding out separately must not shorten the stand-down between
// them - the second refusal arrives while the first is still in force.
func TestPenaltyNeverShortensAnExistingOne(t *testing.T) {
	w, clock := newTestWindow(15, time.Minute)
	w.penalise(65 * time.Second)
	clock.advance(5 * time.Second)
	w.penalise(10 * time.Second)

	wait, ok := w.reserve()
	if ok {
		t.Fatal("a shorter penalty released a longer one still in force")
	}
	if wait != 60*time.Second {
		t.Fatalf("wait = %s, want the 60s remaining of the original penalty", wait)
	}
}

// A caller whose deadline expires waiting here has been deferred, not refused.
// The distinction is what keeps a cached fragment from being read as "this
// account has no animated avatar".
func TestAcquireReportsTheContextErrorRatherThanAdmitting(t *testing.T) {
	w, _ := newTestWindow(1, time.Minute)
	if _, ok := w.reserve(); !ok {
		t.Fatal("the first request should have been admitted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := w.acquire(ctx); err != context.DeadlineExceeded {
		t.Fatalf("acquire err = %v, want context.DeadlineExceeded", err)
	}
}

// Real time throughout, unlike the rest: acquire sleeps on a real timer, so the
// clock it reads has to move on its own.
func TestAcquireAdmitsOnceASlotAgesOut(t *testing.T) {
	w := newRequestWindow(1, 40*time.Millisecond)
	if _, ok := w.reserve(); !ok {
		t.Fatal("the first request should have been admitted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := w.acquire(ctx); err != nil {
		t.Fatalf("acquire err = %v, want a slot once the window slid", err)
	}
}

// 500 is the one that matters: it is what Steam actually sends. A 429 never
// arrives.
func TestMiniprofileRefusalCoversWhatSteamActuallySends(t *testing.T) {
	for _, status := range []int{500, 429, 403, 503} {
		if !isMiniprofileRefusal(status) {
			t.Errorf("status %d should count as a refusal", status)
		}
	}
	for _, status := range []int{200, 304, 404} {
		if isMiniprofileRefusal(status) {
			t.Errorf("status %d is an answer about the account, not a refusal", status)
		}
	}
}
