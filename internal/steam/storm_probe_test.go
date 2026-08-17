package steam

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// This replays a whole account refresh against the real Steam - every endpoint at
// once, in the shape and at the concurrency the app uses - to find which one
// gives out first under combined load, and how long the app is unusable
// afterwards. The ladder probe next door measures endpoints one at a time, which
// is how their ceilings were found; this measures them competing, which is what
// actually happens.
//
//	$env:TCNO_PROBE_IDS = "765...,765...,765..."
//	$env:TCNO_STORM = "1"
//	go test ./internal/steam -run RefreshStormProbe -v -timeout 30m
//
// Close the app first: its own refresh would add requests this cannot count, and
// on the tightest endpoint that is the difference between a clean run and a block.
//
// Two rounds by default, because that is the case worth knowing about. A six
// hourly refresh and an unlock-triggered one landing near each other is ordinary,
// and each round spends a full account's worth of the miniprofile budget - which
// measured out at roughly twenty requests a minute.

const (
	// Per-request budgets mirror the app's own, so a timeout here is a timeout
	// there: profile XML gets its retry policy's attempt timeout, and the
	// miniprofile and avatar contexts come from runProfileRefresh.
	stormProfileTimeout     = 10 * time.Second
	stormMiniprofileTimeout = 15 * time.Second
	stormAvatarTimeout      = 20 * time.Second

	stormRecoveryPoll = 10 * time.Second
	stormRecoveryMax  = 8 * time.Minute
	stormMaxDuration  = 12 * time.Minute
)

// stormStat accumulates one endpoint's behaviour across the whole storm.
type stormStat struct {
	mu        sync.Mutex
	name      string
	timeout   time.Duration
	requests  int
	ok        int
	timeouts  int
	latencies []time.Duration
	statuses  map[int]int
	// firstRefusalAt is the ordinal of the refused request within this endpoint,
	// which is the number that matters for a budget expressed as a count.
	firstRefusalAt      int
	firstRefusalElapsed time.Duration
	firstRefusal        *probeRefusal
	sampleURL           string
}

func newStormStat(name string, timeout time.Duration) *stormStat {
	return &stormStat{name: name, timeout: timeout, statuses: map[int]int{}}
}

func (s *stormStat) record(result probeResponse, url string, since time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests++
	s.latencies = append(s.latencies, result.latency)
	s.statuses[result.status]++
	if s.sampleURL == "" {
		s.sampleURL = url
	}
	if result.refusal != nil && result.refusal.transport != nil &&
		errors.Is(result.refusal.transport, context.DeadlineExceeded) {
		s.timeouts++
	}
	if result.refusal != nil && s.firstRefusal == nil {
		s.firstRefusal = result.refusal
		s.firstRefusalAt = s.requests
		s.firstRefusalElapsed = since
	}
	if result.status >= 200 && result.status < 300 {
		s.ok++
	}
}

func (s *stormStat) percentile(p float64) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.latencies) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), s.latencies...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[int(float64(len(sorted)-1)*p)]
}

func (s *stormStat) histogram() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	codes := make([]int, 0, len(s.statuses))
	for code := range s.statuses {
		codes = append(codes, code)
	}
	sort.Ints(codes)
	parts := make([]string, 0, len(codes))
	for _, code := range codes {
		parts = append(parts, fmt.Sprintf("%d:%d", code, s.statuses[code]))
	}
	return strings.Join(parts, " ")
}

func stormEnvInt(t *testing.T, name string, fallback int) int {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		t.Fatalf("%s must be a positive integer, got %q", name, raw)
	}
	return parsed
}

// stormRequest issues one request under the endpoint's own timeout and files the
// result. The timeout is per endpoint rather than global so that a slow avatar
// and a slow profile page are judged by the same deadlines the app gives them.
func stormRequest(ctx context.Context, client *http.Client, stat *stormStat, url string, since func() time.Duration) probeResponse {
	requestCtx, cancel := context.WithTimeout(ctx, stat.timeout)
	defer cancel()
	result := probeOnce(requestCtx, client, url, 0)
	stat.record(result, url, since())
	return result
}

// newStormHTTPClient keeps one connection pool for the whole storm, sized to the
// concurrency under test so the run measures Steam rather than our own reuse.
//
// Three hosts are in play, so the per-host ceiling is the concurrency rather than
// a share of it - the app has the same shape, and a pool that throttled below
// what is being measured would quietly answer a different question.
func newStormHTTPClient(concurrency int) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = concurrency * 3
	transport.MaxIdleConnsPerHost = concurrency
	// No client-wide Timeout: each request carries the app's own deadline for that
	// endpoint, and a second shorter one here would mask which of them was hit.
	return &http.Client{Transport: transport}
}

func TestSteamRefreshStormProbe(t *testing.T) {
	if strings.TrimSpace(os.Getenv("TCNO_STORM")) == "" {
		t.Skip("set TCNO_STORM=1 to run the combined refresh storm probe")
	}
	ids := probeIDs(t)
	accounts := stormEnvInt(t, "TCNO_STORM_ACCOUNTS", 12)
	concurrency := stormEnvInt(t, "TCNO_STORM_CONCURRENCY", 5)
	rounds := stormEnvInt(t, "TCNO_STORM_ROUNDS", 2)
	gap := time.Duration(stormEnvInt(t, "TCNO_STORM_GAP_SECONDS", 10)) * time.Second

	profile := newStormStat("profile-xml", stormProfileTimeout)
	mini := newStormStat("miniprofile", stormMiniprofileTimeout)
	avatar := newStormStat("avatar-cdn", stormAvatarTimeout)
	stats := []*stormStat{profile, mini, avatar}

	ctx, cancel := context.WithTimeout(context.Background(), stormMaxDuration)
	defer cancel()
	client := newStormHTTPClient(concurrency)
	defer client.CloseIdleConnections()

	t.Logf("storm: %d accounts x %d rounds at concurrency %d, %s between rounds",
		accounts, rounds, concurrency, gap)
	t.Logf("per-request timeouts: profile=%s miniprofile=%s avatar=%s",
		stormProfileTimeout, stormMiniprofileTimeout, stormAvatarTimeout)

	started := time.Now()
	since := func() time.Duration { return time.Since(started) }

	for round := 1; round <= rounds; round++ {
		if round > 1 {
			select {
			case <-ctx.Done():
			case <-time.After(gap):
			}
		}
		if ctx.Err() != nil {
			t.Logf("time budget spent before round %d", round)
			break
		}
		roundStarted := time.Now()
		var wg sync.WaitGroup
		slots := make(chan struct{}, concurrency)
		for i := 0; i < accounts; i++ {
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				i = accounts
				continue
			}
			wg.Add(1)
			go func(seq int) {
				defer wg.Done()
				defer func() { <-slots }()
				id := ids[seq%len(ids)]
				nonce := round*10_000 + seq

				// The same order the refresh uses, because the avatar URL is only
				// known once the profile XML has been read.
				xmlResult := stormRequest(ctx, client, profile,
					bustCache(fmt.Sprintf("https://steamcommunity.com/profiles/%s?xml=1", id), nonce, started), since)

				if formats, err := FormatsFromID64(id); err == nil {
					stormRequest(ctx, client, mini,
						bustCache(fmt.Sprintf("https://steamcommunity.com/miniprofile/%s", formats.ID32), nonce, started), since)
				}

				if avatarURL := stormAvatarURL(xmlResult); avatarURL != "" {
					stormRequest(ctx, client, avatar, bustCache(avatarURL, nonce, started), since)
				}
			}(i)
		}
		wg.Wait()
		t.Logf("round %d finished in %s", round, time.Since(roundStarted).Round(time.Millisecond))
	}

	total := time.Since(started)
	t.Logf("--- storm finished in %s ---", total.Round(time.Millisecond))
	totalRequests := 0
	for _, stat := range stats {
		totalRequests += stat.requests
		t.Logf("%-12s requests=%-3d ok=%-3d timeouts=%-2d p50=%-8s p95=%-8s max=%-8s status[%s]",
			stat.name, stat.requests, stat.ok, stat.timeouts,
			stat.percentile(0.50).Round(time.Millisecond),
			stat.percentile(0.95).Round(time.Millisecond),
			stat.percentile(1).Round(time.Millisecond),
			stat.histogram())
		if stat.firstRefusal == nil {
			continue
		}
		r := stat.firstRefusal
		if r.transport != nil {
			t.Logf("  REFUSED at its request #%d (%s into the storm): %v",
				stat.firstRefusalAt, stat.firstRefusalElapsed.Round(time.Millisecond), r.transport)
			continue
		}
		t.Logf("  REFUSED at its request #%d (%s into the storm): HTTP %d retryAfter=%q body=%q",
			stat.firstRefusalAt, stat.firstRefusalElapsed.Round(time.Millisecond),
			r.status, r.retryAfter, r.bodyHead)
	}
	t.Logf("%d requests total, %.1f req/s across all endpoints",
		totalRequests, float64(totalRequests)/total.Seconds())

	// Recovery last and one endpoint at a time: polling them together would let a
	// still-blocked endpoint's requests count against a neighbour's budget.
	for _, stat := range stats {
		if stat.firstRefusal == nil {
			continue
		}
		stormMeasureRecovery(t, client, stat)
	}
}

// stormAvatarURL pulls the full-size avatar out of a profile XML response body.
func stormAvatarURL(result probeResponse) string {
	if len(result.body) == 0 {
		return ""
	}
	doc, err := parseProfileXMLDoc(result.body)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(doc.AvatarFull)
}

func stormMeasureRecovery(t *testing.T, client *http.Client, stat *stormStat) {
	t.Helper()
	if stat.sampleURL == "" {
		return
	}
	t.Logf("%s: waiting out the refusal, one request every %s (giving up after %s)",
		stat.name, stormRecoveryPoll, stormRecoveryMax)
	ctx, cancel := context.WithTimeout(context.Background(), stormRecoveryMax)
	defer cancel()
	started := time.Now()
	for attempt := 1; ; attempt++ {
		select {
		case <-ctx.Done():
			t.Logf("%s: STILL REFUSING after %s - longer than this probe waits", stat.name, stormRecoveryMax)
			return
		case <-time.After(stormRecoveryPoll):
		}
		requestCtx, requestCancel := context.WithTimeout(ctx, stat.timeout)
		result := probeOnce(requestCtx, client, bustCache(stat.sampleURL, attempt, time.Now()), 0)
		requestCancel()
		if result.refusal == nil && result.status >= 200 && result.status < 300 {
			t.Logf("%s: RECOVERED after %s (%d polls)", stat.name, time.Since(started).Round(time.Second), attempt)
			return
		}
		t.Logf("  %s still refused after %s (status %d)", stat.name, time.Since(started).Round(time.Second), result.status)
	}
}
