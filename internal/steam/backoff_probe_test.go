package steam

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// This finds how long to stay quiet after Steam refuses, which is the one number
// a backoff needs and the one the other probes cannot give.
//
//	$env:TCNO_PROBE_IDS = "765...,765..."
//	$env:TCNO_BACKOFF = "1"
//	go test ./internal/steam -run BackoffProbe -v -timeout 30m
//
// The storm probe measured recovery by asking every ten seconds and got 52s and
// 62s. Those are upper bounds at best and may be self-inflicted: Steam's harsher
// rate limiting is widely reported to restart its clock when you keep asking
// during a block, so a dense poll can hold a door shut that it is trying to open.
//
// So this one goes completely silent first, then asks once. If the wall clears
// sooner than the polled runs suggested, the polling was the problem and the app
// must never retry into a refusal - it must wait out a fixed period and only then
// try, exactly once.
//
// Run it more than once at different TCNO_BACKOFF_QUIET_SECONDS to bracket the
// answer. Each run costs one deliberate trip of the limit.

const (
	// backoffTripCap bounds the deliberate trip. The limit measured at twenty, so
	// this is enough to cross it and not much more - the point is to be refused
	// once, not to lean on it.
	backoffTripCap     = 26
	backoffProbeMax    = 20 * time.Minute
	backoffRequestWait = 15 * time.Second
)

func backoffEnvSeconds(t *testing.T, name string, fallback time.Duration) time.Duration {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	seconds, err := time.ParseDuration(raw + "s")
	if err != nil || seconds <= 0 {
		t.Fatalf("%s must be a positive number of seconds, got %q", name, raw)
	}
	return seconds
}

func TestSteamMiniprofileBackoffProbe(t *testing.T) {
	if strings.TrimSpace(os.Getenv("TCNO_BACKOFF")) == "" {
		t.Skip("set TCNO_BACKOFF=1 to run the miniprofile backoff probe")
	}
	ids := probeIDs(t)
	quiet := backoffEnvSeconds(t, "TCNO_BACKOFF_QUIET_SECONDS", 30*time.Second)
	step := backoffEnvSeconds(t, "TCNO_BACKOFF_STEP_SECONDS", 15*time.Second)

	urls := probeEndpoints["miniprofile"].urls(t, ids)
	ctx, cancel := context.WithTimeout(context.Background(), backoffProbeMax)
	defer cancel()
	client := &http.Client{Timeout: backoffRequestWait}
	defer client.CloseIdleConnections()

	t.Logf("tripping the miniprofile limit (cap %d requests), then going silent for %s",
		backoffTripCap, quiet)

	// Trip it. Serial on purpose: the limit measured as a count rather than a
	// rate, so concurrency would only make the moment of refusal harder to place.
	tripStarted := time.Now()
	tripped := 0
	var refusedAt time.Duration
	for i := 0; i < backoffTripCap; i++ {
		if ctx.Err() != nil {
			t.Fatal("time budget spent before the limit was tripped")
		}
		result := probeOnce(ctx, client, bustCache(urls[i%len(urls)], i, tripStarted), 0)
		tripped++
		if result.refusal != nil {
			refusedAt = time.Since(tripStarted)
			t.Logf("refused at request #%d after %s (status %d)",
				tripped, refusedAt.Round(time.Millisecond), result.status)
			break
		}
	}
	if refusedAt == 0 {
		t.Skipf("%d requests did not trip the limit; the bucket may still be draining "+
			"from an earlier run - leave a few minutes and try again", tripped)
	}

	// The whole point: nothing at all goes to Steam for this long.
	blockStarted := time.Now()
	t.Logf("silent for %s...", quiet)
	select {
	case <-ctx.Done():
		t.Fatal("time budget spent during the quiet period")
	case <-time.After(quiet):
	}

	for attempt := 1; ; attempt++ {
		waited := time.Since(blockStarted)
		result := probeOnce(ctx, client, bustCache(urls[0], 1_000+attempt, time.Now()), 0)
		if result.refusal == nil && result.status >= 200 && result.status < 300 {
			t.Logf("CLEARED after %s of silence, on probe request %d",
				waited.Round(time.Second), attempt)
			t.Log("compare with the storm probe's polled recovery: if this is materially " +
				"shorter, asking during a block is what was holding it shut")
			return
		}
		t.Logf("  still refused after %s of near-silence (status %d, %d probe requests so far)",
			waited.Round(time.Second), result.status, attempt)
		if ctx.Err() != nil {
			t.Logf("gave up after %s", waited.Round(time.Second))
			return
		}
		select {
		case <-ctx.Done():
			t.Logf("gave up after %s", time.Since(blockStarted).Round(time.Second))
			return
		case <-time.After(step):
		}
	}
}
