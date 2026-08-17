package steam

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
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
	probeStepRequests  = 40
	probeRequestBudget = 15 * time.Second
)

// Profile XML is served with Cache-Control: public,max-age=3600, so asking for
// the same account twice inside the hour is answered by the Akamai edge and
// never reaches Steam. A first pass at this probe cycled three accounts and
// reported no refusal at 456 requests a second - which was the CDN's throughput
// for three cached objects, and said nothing whatever about a rate limit.
//
// So every request carries a nonce and is therefore an origin fetch. That is not
// what the app does for one account, but it is what the app does across a list:
// a dozen accounts are a dozen distinct URLs, and each is origin work. Set
// TCNO_PROBE_CACHE=allow to drop the nonce and measure the cached path instead -
// worth knowing, since it is what a second refresh within the hour actually pays.
//
// freshWindow below is the check on all of this: Expires-Date equal to the full
// max-age means the response was generated for us, and anything less means the
// edge answered from a copy it already had.
const probeMaxAge = time.Hour

// probeConcurrencies is the ladder. It starts below what the app does today and
// ends well above it, so the answer is bracketed rather than merely bounded.
var probeConcurrencies = []int{1, 2, 3, 5, 8, 12}

type probeOutcome struct {
	concurrency int
	requests    int
	ok          int
	elapsed     time.Duration
	latencies   []time.Duration
	// origin counts responses Steam generated for us rather than the edge
	// replaying one it held. A rung that is mostly not origin is measuring Akamai.
	// cacheKnown is how many responses carried the headers needed to tell, since
	// not every endpoint does and "unknown" must not be reported as "cached".
	origin     int
	cacheKnown int
	// statuses is every status seen, because a rung reported only as "ok=20 of 40"
	// says nothing about what the other twenty were - and the first run of this
	// probe against the miniprofile endpoint produced exactly that, leaving the
	// interesting half of the result unrecoverable.
	statuses map[int]int
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
//
// Every 5xx counts, and that is not over-broad: the miniprofile endpoint states
// its limit as a plain 500, so a set naming only the polite codes - 429, 503 -
// watched twenty refusals in a row go by and called the rung clean.
func isRefusal(status int) bool {
	if status >= 500 {
		return true
	}
	return status == http.StatusTooManyRequests || status == http.StatusForbidden
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

func probeCacheAllowed() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("TCNO_PROBE_CACHE")), "allow")
}

// bustCache appends a nonce so the edge has nothing to replay.
func bustCache(url string, sequence int) string {
	if probeCacheAllowed() {
		return url
	}
	separator := "?"
	if strings.Contains(url, "?") {
		separator = "&"
	}
	return fmt.Sprintf("%s%stcnoprobe=%d-%d", url, separator, time.Now().UnixNano(), sequence)
}

// servedFromOrigin reports whether Steam generated this response rather than the
// edge replaying one. A cached copy has already spent part of its lifetime, so
// its remaining freshness is short of the full max-age.
//
// known is false when the response carries no such pair to compare - the
// miniprofile endpoint is one - and the caller must then say "unknown" rather
// than "cached", which is a different claim and a wrong one.
func servedFromOrigin(header http.Header) (origin, known bool) {
	date, err := http.ParseTime(header.Get("Date"))
	if err != nil {
		return false, false
	}
	expires, err := http.ParseTime(header.Get("Expires"))
	if err != nil {
		return false, false
	}
	// A second of slack: Date and Expires are whole seconds and can straddle a tick.
	return expires.Sub(date) >= probeMaxAge-time.Second, true
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

type probeResult struct {
	latency    time.Duration
	status     int
	origin     bool
	cacheKnown bool
	refusal    *probeRefusal
}

func probeOnce(ctx context.Context, client *http.Client, url string) probeResult {
	started := time.Now()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return probeResult{refusal: &probeRefusal{transport: err}}
	}
	request.Header.Set("User-Agent", "TcNo Account Switcher")
	response, err := client.Do(request)
	if err != nil {
		return probeResult{latency: time.Since(started), refusal: &probeRefusal{transport: err}}
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	latency := time.Since(started)
	origin, cacheKnown := servedFromOrigin(response.Header)
	result := probeResult{
		latency: latency, status: response.StatusCode, origin: origin, cacheKnown: cacheKnown,
	}
	if !isRefusal(response.StatusCode) {
		return result
	}
	head := strings.Join(strings.Fields(string(body)), " ")
	if len(head) > 200 {
		head = head[:200]
	}
	result.refusal = &probeRefusal{
		status:     response.StatusCode,
		retryAfter: response.Header.Get("Retry-After"),
		bodyHead:   head,
	}
	return result
}

// statusHistogram renders the rung's statuses lowest first, so an unexpected one
// is impossible to miss. Status 0 is a transport error.
func (o probeOutcome) statusHistogram() string {
	codes := make([]int, 0, len(o.statuses))
	for code := range o.statuses {
		codes = append(codes, code)
	}
	sort.Ints(codes)
	parts := make([]string, 0, len(codes))
	for _, code := range codes {
		parts = append(parts, fmt.Sprintf("%d:%d", code, o.statuses[code]))
	}
	return strings.Join(parts, " ")
}

func probeRung(ctx context.Context, client *http.Client, urls []string, concurrency, requests int) probeOutcome {
	outcome := probeOutcome{concurrency: concurrency, statuses: map[int]int{}}
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
			result := probeOnce(ctx, client, url)
			mu.Lock()
			defer mu.Unlock()
			outcome.requests++
			outcome.latencies = append(outcome.latencies, result.latency)
			outcome.statuses[result.status]++
			if result.cacheKnown {
				outcome.cacheKnown++
				if result.origin {
					outcome.origin++
				}
			}
			if result.refusal != nil {
				if outcome.refusal == nil {
					outcome.refusal = result.refusal
				}
				return
			}
			if result.status >= 200 && result.status < 300 {
				outcome.ok++
			}
		}(bustCache(urls[i%len(urls)], i))
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

	// TCNO_PROBE_MAX narrows the whole run. Once an endpoint is known to refuse,
	// the useful question is where the transition is, and answering it with sixty
	// requests rather than the full budget is both faster and better manners.
	budget := probeMaxRequests
	if raw := strings.TrimSpace(os.Getenv("TCNO_PROBE_MAX")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			t.Fatalf("TCNO_PROBE_MAX must be a positive integer, got %q", raw)
		}
		budget = parsed
	}

	mode := "cache-busted (origin)"
	if probeCacheAllowed() {
		mode = "cache allowed (edge may answer)"
	}
	t.Logf("probing %d URLs %s, ladder %v, at most %d requests over %s",
		len(urls), mode, probeConcurrencies, budget, probeMaxDuration)

	spent := 0
	var lastClean *probeOutcome
	for _, concurrency := range probeConcurrencies {
		remaining := budget - spent
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
		origin := fmt.Sprintf("%d", outcome.origin)
		if outcome.cacheKnown == 0 {
			origin = "n/a"
		}
		t.Logf("concurrency=%-2d requests=%-3d ok=%-3d origin=%-3s rate=%5.1f/s p50=%-8s p95=%-8s status[%s]",
			outcome.concurrency, outcome.requests, outcome.ok, origin, outcome.rate(),
			outcome.percentile(0.50).Round(time.Millisecond),
			outcome.percentile(0.95).Round(time.Millisecond),
			outcome.statusHistogram())
		// Not every non-2xx is a refusal, and one that is neither is the shape most
		// likely to be misread as a clean rung.
		if other := outcome.requests - outcome.ok; other > 0 && outcome.refusal == nil {
			t.Logf("  NOTE: %d/%d responses were neither 2xx nor a recognised refusal; see the status column",
				other, outcome.requests)
		}
		// Said per rung rather than once at the end, because a rung the edge
		// answered is not evidence about a limit and must not read like it. Only
		// claimed where the headers actually settle the question.
		if cached := outcome.cacheKnown - outcome.origin; !probeCacheAllowed() && cached > 0 {
			t.Logf("  WARNING: %d/%d responses came from cache; this rung does not measure Steam",
				cached, outcome.requests)
		}

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
		t.Log("this is 'not right now', not 'no limit': Steam publishes none, and " +
			"community throttling varies by time of day and by edge")
	}
}
