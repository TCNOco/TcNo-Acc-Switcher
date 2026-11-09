package steam

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

func TestProfileRefreshRetryTimingAndCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		started := time.Now()
		attempts := 0
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		_, err := fetchProfileXMLWithRetry(ctx, profileXMLRetryPolicy{
			MaxAttempts: 3, AttemptTimeout: 5 * time.Second, Delay: time.Second,
		}, func(attempt context.Context) (ProfileXMLFields, error) {
			attempts++
			<-attempt.Done()
			return ProfileXMLFields{}, attempt.Err()
		}, nil)
		if !errors.Is(err, context.DeadlineExceeded) || attempts != 2 || time.Since(started) != 12*time.Second {
			t.Fatalf("retry deadline err=%v attempts=%d elapsed=%v", err, attempts, time.Since(started))
		}
	})
}
