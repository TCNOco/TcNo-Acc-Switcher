package steam

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// This file measures where steamcommunity.com starts refusing, so the account
// refresh can be paced against a number rather than a guess. It talks to the real
// Steam, so it never runs unless asked:
//
//	TCNO_PROBE_IDS=765...,765... go test ./internal/steam -run RateLimitProbe -v
//
// Only the unauthenticated pages the refresh already reads are probed - profile
// XML and miniprofile. The authenticated GCPD read the CS2 sweep makes is
// deliberately not here: it is the one endpoint where being turned away is
// attached to an account rather than an address, and walking into that wall on
// purpose is how a short ban becomes a long one. What that limit looks like is
// instead recorded whenever ordinary use meets it, by rateLimitEvidence in the
// steamguard package.
//
// Everything here is bounded twice over, by request count and by wall clock, and
// stops at the first sign of a refusal rather than pushing past it to find the
// shape of the wall.

const (
	probeMaxRequests = 240
	probeMaxDuration = 3 * time.Minute
	// probeStepRequests is per rung of the ladder. Enough to be more than noise,
	// few enough that a rung which is already over the line is cheap.
	probeStepRequests  = 24
	probeRequestBudget = 15 * time.Second
)

// probeConcurrencies is the ladder. It starts below what the app does today and
// ends well above it, so the answer is bracketed rather than merely bounded.
var probeConcurrencies = []int{1, 2, 3, 5, 8, 12}

type probeOutcome struct {
	concurrency int
	requests    int
	ok          int
	elapsed     time.Duration
	latencies   []time.Duration
	// refusal is the first response that looked like a limit rather than an
	// answer. Its presence ends the whole probe.
	refusal *probeRefusal
}

type probeRefusal struct {
	status     int
	retryAfter string
	transport  error
	bodyHead   string
}

func (o probeOutcome) rate() float64 {
	if o.elapsed <= 0 {
		return 0
	}
	return float64(o.requests) / o.elapsed.Seconds()
}

func (o probeOutcome) percentile(p float64) time.Duration {
	if len(o.latencies) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), o.latencies...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	index := int(float64(len(sorted)-1) * p)
	return sorted[index]
}

// isRefusal separates "Steam declined to serve this" from "that profile does not
// exist". A 404 is an answer about an account; the rest are answers about us.
func isRefusal(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusForbidden,
		http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusGatewayTimeout:
		return true
	}
	return false
}

func probeIDs(t *testing.T) []string {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv("TCNO_PROBE_IDS"))
	if raw == "" {
		t.Skip("set TCNO_PROBE_IDS to a comma-separated list of SteamID64s to run the rate limit probe")
	}
	var ids []string
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			ids = append(ids, trimmed)
		}
	}
	if len(ids) < 2 {
		t.Skip("the probe needs at least two SteamID64s, or it measures one cached URL rather than a limit")
	}
	return ids
}

// probeURL builds the same URL the refresh would ask for. TCNO_PROBE_ENDPOINT
// picks which of the two pages to measure; they are served by different parts of
// Steam and need not share a limit.
func probeURL(t *testing.T, steamID64 string) string {
	t.Helper()
	switch strings.ToLower(strings.TrimSpace(os.Getenv("TCNO_PROBE_ENDPOINT"))) {
	case "", "profile":
		return fmt.Sprintf("https://steamcommunity.com/profiles/%s?xml=1", steamID64)
	case "miniprofile":
		formats, err := FormatsFromID64(steamID64)
		if err != nil {
			t.Fatalf("steam id %s: %v", steamID64, err)
		}
		return fmt.Sprintf("https://steamcommunity.com/miniprofile/%s", formats.ID32)
	default:
		t.Fatal(`TCNO_PROBE_ENDPOINT must be "profile" or "miniprofile"`)
		return ""
	}
}

// probeClient pools as many connections as the widest rung, so the ladder
// measures Steam rather than our own connection reuse.
func probeClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	widest := probeConcurrencies[len(probeConcurrencies)-1]
	transport.MaxIdleConns = widest * 2
	transport.MaxIdleConnsPerHost = widest
	return &http.Client{Transport: transport, Timeout: probeRequestBudget}
}

func probeOnce(ctx context.Context, client *http.Client, url string) (time.Duration, int, *probeRefusal) {
	started := time.Now()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, &probeRefusal{transport: err}
	}
	request.Header.Set("User-Agent", "TcNo Account Switcher")
	response, err := client.Do(request)
	if err != nil {
		return time.Since(started), 0, &probeRefusal{transport: err}
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	latency := time.Since(started)
	if !isRefusal(response.StatusCode) {
		return latency, response.StatusCode, nil
	}
	head := strings.Join(strings.Fields(string(body)), " ")
	if len(head) > 200 {
		head = head[:200]
	}
	return latency, response.StatusCode, &probeRefusal{
		status:     response.StatusCode,
		retryAfter: response.Header.Get("Retry-After"),
		bodyHead:   head,
	}
}

func probeRung(ctx context.Context, client *http.Client, urls []string, concurrency, requests int) probeOutcome {
	outcome := probeOutcome{concurrency: concurrency}
	var mu sync.Mutex
	var wg sync.WaitGroup
	slots := make(chan struct{}, concurrency)
	started := time.Now()

	for i := 0; i < requests; i++ {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			i = requests
			continue
		}
		mu.Lock()
		stop := outcome.refusal != nil
		mu.Unlock()
		if stop {
			<-slots
			break
		}
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			defer func() { <-slots }()
			latency, status, refusal := probeOnce(ctx, client, url)
			mu.Lock()
			defer mu.Unlock()
			outcome.requests++
			outcome.latencies = append(outcome.latencies, latency)
			if refusal != nil {
				if outcome.refusal == nil {
					outcome.refusal = refusal
				}
				return
			}
			if status >= 200 && status < 300 {
				outcome.ok++
			}
		}(urls[i%len(urls)])
	}
	wg.Wait()
	outcome.elapsed = time.Since(started)
	return outcome
}

// TestSteamCommunityRateLimitProbe walks a ladder of concurrency levels against
// the community pages the refresh reads, and stops at the first rung Steam
// refuses. The rung below it is the budget worth pacing to.
func TestSteamCommunityRateLimitProbe(t *testing.T) {
	ids := probeIDs(t)
	urls := make([]string, 0, len(ids))
	for _, id := range ids {
		urls = append(urls, probeURL(t, id))
	}

	ctx, cancel := context.WithTimeout(context.Background(), probeMaxDuration)
	defer cancel()
	client := probeClient()
	defer client.CloseIdleConnections()

	t.Logf("probing %d URLs, ladder %v, at most %d requests over %s",
		len(urls), probeConcurrencies, probeMaxRequests, probeMaxDuration)

	spent := 0
	var lastClean *probeOutcome
	for _, concurrency := range probeConcurrencies {
		remaining := probeMaxRequests - spent
		if remaining <= 0 {
			t.Logf("request budget spent after %d requests; ladder stopped early", spent)
			break
		}
		requests := probeStepRequests
		if requests > remaining {
			requests = remaining
		}

		outcome := probeRung(ctx, client, urls, concurrency, requests)
		spent += outcome.requests
		t.Logf("concurrency=%-2d requests=%-3d ok=%-3d rate=%5.1f/s p50=%-8s p95=%s",
			outcome.concurrency, outcome.requests, outcome.ok, outcome.rate(),
			outcome.percentile(0.50).Round(time.Millisecond),
			outcome.percentile(0.95).Round(time.Millisecond))

		if outcome.refusal != nil {
			r := outcome.refusal
			if r.transport != nil {
				t.Logf("REFUSED at concurrency=%d after %d requests: transport error: %v",
					concurrency, spent, r.transport)
			} else {
				t.Logf("REFUSED at concurrency=%d after %d requests: HTTP %d retryAfter=%q body=%q",
					concurrency, spent, r.status, r.retryAfter, r.bodyHead)
			}
			if lastClean != nil {
				t.Logf("last clean rung: concurrency=%d at %.1f req/s - pace below this",
					lastClean.concurrency, lastClean.rate())
			} else {
				t.Log("no rung came back clean; the limit is at or below the narrowest step")
			}
			return
		}
		if ctx.Err() != nil {
			t.Logf("time budget spent after %d requests; ladder stopped early", spent)
			return
		}
		clean := outcome
		lastClean = &clean
	}

	if lastClean != nil {
		t.Logf("no refusal within budget; cleared concurrency=%d at %.1f req/s over %d requests",
			lastClean.concurrency, lastClean.rate(), spent)
	}
}
