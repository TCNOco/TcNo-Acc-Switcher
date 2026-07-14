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
	return false
}
