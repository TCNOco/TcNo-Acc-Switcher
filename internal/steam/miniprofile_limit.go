package steam

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Steam meters the miniprofile endpoint far more tightly than anything else the
// refresh touches, publishes nothing about it, and says no with a bare HTTP 500
// carrying no Retry-After. Measured against the live service: it refuses at the
// twenty-first request inside a minute, and serves 500s for the fifty to sixty
// seconds that follow.
//
// The profile XML on the same hostname cleared 240 requests at 26 a second
// without complaint, and the avatar CDN never refused at all. So this leash
// belongs to one endpoint and must not be put on the others; applying a shared
// rate to all three would make a four second refresh take a minute for no reason.
const (
	miniprofileWindow = time.Minute

	// miniprofileBudget is fifteen of the measured twenty. The margin covers
	// requests this process cannot see and must not assume away: the limit is per
	// address, so a second copy of the switcher, a browser on the same network, or
	// anything else asking Steam at the same moment spends from the same twenty.
	miniprofileBudget = 15

	// miniprofilePenalty is how long to stand down once Steam has actually
	// refused. Retrying inside the block neither shortens nor lengthens it, so it
	// waits the window out once, with a few seconds to spare.
	miniprofilePenalty = 65 * time.Second
)

// requestWindow admits at most capacity requests in any window, and can be told
// to stand down entirely for a while.
//
// A sliding window rather than a fixed gap between requests, because the refresh
// is bursty by nature: a dozen accounts arrive together when a page opens, and
// they fit inside one window with room over. Pacing them evenly to respect a
// sustained-rate rule would turn a four second refresh into forty-eight and buy
// nothing. What must not happen is a second burst inside the same minute.
type requestWindow struct {
	mu       sync.Mutex
	capacity int
	window   time.Duration
	// stamps holds the admitted requests still inside the window, oldest first.
	stamps       []time.Time
	blockedUntil time.Time
	now          func() time.Time
}

func newRequestWindow(capacity int, window time.Duration) *requestWindow {
	return &requestWindow{capacity: capacity, window: window, now: time.Now}
}

var miniprofileLimiter = newRequestWindow(miniprofileBudget, miniprofileWindow)

// reserve takes a slot if one is free, or reports how long until the next one is.
func (w *requestWindow) reserve() (time.Duration, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.now()
	if now.Before(w.blockedUntil) {
		return w.blockedUntil.Sub(now), false
	}
	cutoff := now.Add(-w.window)
	kept := w.stamps[:0]
	for _, stamp := range w.stamps {
		if stamp.After(cutoff) {
			kept = append(kept, stamp)
		}
	}
	w.stamps = kept
	if len(w.stamps) < w.capacity {
		w.stamps = append(w.stamps, now)
		return 0, true
	}
	// The oldest admitted request is the first to leave the window.
	return w.stamps[0].Add(w.window).Sub(now), false
}

// acquire blocks until a slot is free or ctx ends.
//
// A caller whose deadline expires waiting here has been deferred, not refused,
// and the difference matters upstream: a deferred miniprofile must leave the
// account's cached fragment alone rather than be read as "this account has no
// animated avatar".
func (w *requestWindow) acquire(ctx context.Context) error {
	for {
		wait, ok := w.reserve()
		if ok {
			return nil
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// penalise stands the endpoint down after Steam has refused. Extending an
// existing block, never shortening it.
func (w *requestWindow) penalise(d time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	until := w.now().Add(d)
	if until.After(w.blockedUntil) {
		w.blockedUntil = until
	}
}

// isMiniprofileRefusal reports whether a status is Steam declining to serve us
// rather than answering about the account.
//
// 500 is in the list because that is what the endpoint actually returns when it
// has had enough, not 429, which is what a reader would expect. 429 is here too
// because it is the harsher tier, reported to last hours rather than a minute.
func isMiniprofileRefusal(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusForbidden,
		http.StatusInternalServerError, http.StatusServiceUnavailable:
		return true
	}
	return false
}
