package steam

import (
	"context"
	"errors"
	"net"
	"net/url"
	"time"
)

const temporaryProfileRefreshMessage = "Steam profile is temporarily unavailable. Please try again later."

type profileXMLRetryPolicy struct {
	MaxAttempts    int
	AttemptTimeout time.Duration
	Delay          time.Duration
}

var defaultProfileXMLRetryPolicy = profileXMLRetryPolicy{
	MaxAttempts:    3,
	AttemptTimeout: 10 * time.Second,
	Delay:          500 * time.Millisecond,
}

// profileRefreshRetryDelays is the backoff between whole refresh rounds after
// Steam could not be reached at all.
//
// The policy above only covers the attempts for one account inside one round.
// A machine resuming from sleep runs the refresh before its network adapter is
// up and every account fails at once; with the account list already cached
// nothing else ever asks for another round, so without this the failure sits on
// every tile until something unrelated happens to trigger one.
var profileRefreshRetryDelays = []time.Duration{
	30 * time.Second,
	time.Minute,
	2 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
}

// profileRefreshQuietFailures is how many consecutive unreachable rounds pass
// before the tiles say so. The first one is the resume-from-sleep case and
// clears itself within the minute, so reporting it is noise the user cannot act
// on; a network that is genuinely gone still gets said out loud.
const profileRefreshQuietFailures = 2

// profileRefreshRetryDelay is the wait after `failures` consecutive unreachable
// rounds, counting the first as 1.
func profileRefreshRetryDelay(failures int) time.Duration {
	if failures < 1 {
		failures = 1
	}
	if failures > len(profileRefreshRetryDelays) {
		failures = len(profileRefreshRetryDelays)
	}
	return profileRefreshRetryDelays[failures-1]
}

func fetchProfileXMLWithRetry(
	ctx context.Context,
	policy profileXMLRetryPolicy,
	fetch func(context.Context) (ProfileXMLFields, error),
	onRetry func(error),
) (ProfileXMLFields, error) {
	attempts := policy.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		attemptCtx := ctx
		cancel := func() {}
		if policy.AttemptTimeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, policy.AttemptTimeout)
		}
		fields, err := fetch(attemptCtx)
		cancel()
		if err == nil {
			return fields, nil
		}
		lastErr = err
		if !isTransientProfileRefreshError(err) || attempt == attempts {
			return ProfileXMLFields{}, err
		}
		if onRetry != nil {
			onRetry(err)
		}
		if policy.Delay <= 0 {
			continue
		}
		timer := time.NewTimer(policy.Delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ProfileXMLFields{}, ctx.Err()
		case <-timer.C:
		}
	}
	return ProfileXMLFields{}, lastErr
}

func profileRefreshErrorState(err error, retrying bool) (message string, pending bool) {
	if retrying {
		return "", true
	}
	if isTransientProfileRefreshError(err) {
		return temporaryProfileRefreshMessage, false
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return temporaryProfileRefreshMessage, false
	}
	return err.Error(), false
}

func isTransientProfileRefreshError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}
	var httpErr *profileXMLHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == 408 || httpErr.StatusCode == 425 || httpErr.StatusCode == 429 || httpErr.StatusCode >= 500
	}
	// Everything net/http reports before it has a response arrives as a
	// *url.Error: DNS lookup, dial, TLS. A DNS "no such host" reports neither
	// Timeout nor Temporary, but it is not a verdict about the account: on a
	// machine that has just woken it means the adapter is not up yet.
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return true
	}
	// A body that did not parse says the same thing from one layer up.
	var bodyErr *profileXMLBodyError
	return errors.As(err, &bodyErr)
}
