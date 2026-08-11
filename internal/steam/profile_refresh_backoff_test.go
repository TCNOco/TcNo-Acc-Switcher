package steam

import (
	"context"
	"net"
	"net/url"
	"testing"
	"time"
)

// dnsFailure is what a machine that has resumed from sleep before its network
// adapter is up gets back from every lookup at once.
func dnsFailure() error {
	return &url.Error{
		Op:  "Get",
		URL: "https://steamcommunity.com/profiles/76561198000000000?xml=1",
		Err: &net.DNSError{Err: "no such host", Name: "steamcommunity.com", IsNotFound: true},
	}
}

// The regression: net.DNSError reports neither Timeout nor Temporary for
// "no such host", so it used to be read as a verdict about the account - one
// attempt, no retry, and the failure painted on every tile until something
// unrelated triggered another round.
func TestDNSFailureIsTransient(t *testing.T) {
	if !isTransientProfileRefreshError(dnsFailure()) {
		t.Fatal("a DNS lookup failure must be retried, not reported as the account's state")
	}
}

func TestUnreadableBodyIsTransient(t *testing.T) {
	err := &profileXMLBodyError{err: context.DeadlineExceeded}
	if !isTransientProfileRefreshError(err) {
		t.Fatal("an unparseable body must be retried")
	}
}

func TestCancelledRefreshIsNotTransient(t *testing.T) {
	err := &url.Error{Op: "Get", URL: "https://steamcommunity.com", Err: context.Canceled}
	if isTransientProfileRefreshError(err) {
		t.Fatal("a cancelled request must not be retried")
	}
}

func TestFetchProfileXMLWithRetryRetriesDNSFailure(t *testing.T) {
	attempts := 0
	want := ProfileXMLFields{SteamID64: "76561198000000000"}

	got, err := fetchProfileXMLWithRetry(
		context.Background(),
		profileXMLRetryPolicy{MaxAttempts: 3, AttemptTimeout: time.Second},
		func(context.Context) (ProfileXMLFields, error) {
			attempts++
			if attempts < 3 {
				return ProfileXMLFields{}, dnsFailure()
			}
			return want, nil
		},
		func(error) {},
	)
	if err != nil {
		t.Fatalf("fetchProfileXMLWithRetry: %v", err)
	}
	if got != want || attempts != 3 {
		t.Fatalf("fields/attempts = %#v/%d, want %#v/3", got, attempts, want)
	}
}

func TestProfileRefreshRetryDelayGrowsAndCaps(t *testing.T) {
	last := time.Duration(0)
	for failures := 1; failures <= len(profileRefreshRetryDelays); failures++ {
		got := profileRefreshRetryDelay(failures)
		if got <= last {
			t.Fatalf("delay(%d) = %v, want more than the previous %v", failures, got, last)
		}
		last = got
	}
	capped := profileRefreshRetryDelay(len(profileRefreshRetryDelays) + 50)
	if capped != last {
		t.Fatalf("delay past the end = %v, want the cap %v", capped, last)
	}
}

func TestProfileRefreshOutcomeSchedulesRetryAndGoesLoudOnlyAfterQuietRounds(t *testing.T) {
	svc := NewSteamService()
	t.Cleanup(func() {
		svc.refreshMu.Lock()
		svc.cancelProfileRefreshRetryLocked()
		svc.refreshMu.Unlock()
	})

	if !svc.profileRefreshQuiet() {
		t.Fatal("the first round must not paint an error on every tile")
	}
	for i := 0; i < profileRefreshQuietFailures; i++ {
		svc.noteProfileRefreshOutcome(true)
	}
	if svc.profileRefreshQuiet() {
		t.Fatalf("after %d unreachable rounds the user has to be told", profileRefreshQuietFailures)
	}

	svc.refreshMu.Lock()
	scheduled := svc.refreshRetry != nil
	svc.refreshMu.Unlock()
	if !scheduled {
		t.Fatal("an unreachable round must schedule the next one; nothing else will")
	}

	svc.noteProfileRefreshOutcome(false)
	svc.refreshMu.Lock()
	stillScheduled := svc.refreshRetry != nil
	failures := svc.refreshFailures
	svc.refreshMu.Unlock()
	if stillScheduled || failures != 0 {
		t.Fatalf("a reachable round must clear the retry and the streak; got scheduled=%t failures=%d", stillScheduled, failures)
	}
	if !svc.profileRefreshQuiet() {
		t.Fatal("the streak must reset once Steam answers again")
	}
}
