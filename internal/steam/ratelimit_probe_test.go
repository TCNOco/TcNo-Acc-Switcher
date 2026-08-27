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

// This measures where each Steam host the account refresh uses starts refusing,
// so concurrency can be set per endpoint from a number rather than from taste.
// It talks to the real Steam, so it never runs unless asked:
//
//	$env:TCNO_PROBE_IDS = "765...,765...,765..."
//	$env:TCNO_PROBE_ENDPOINT = "profile"     # or miniprofile, avatar
//	go test ./internal/steam -run RateLimitProbe -v -timeout 20m
//
// Run one endpoint at a time, and leave a few minutes between runs. They are
// different services behind one name, they refuse independently, and a block
// still in force from the last run makes the next one unreadable.
//
// Close the app first, for the same reason: its own refresh adds requests this
// cannot see or count.
//
// Only the unauthenticated pages are here. The authenticated GCPD read has its
// own probe next door in the steamguard package, because being turned away there
// is attached to an account rather than an address.

const (
	probeMaxRequests = 300
	probeMaxDuration = 4 * time.Minute
	// probeStepRequests is per rung. Deliberately larger than the ~20 that first
	// tripped the miniprofile endpoint, so a count-based limit falls inside a rung
	// rather than straddling two and looking like noise.
	probeStepRequests  = 30
	probeRequestBudget = 15 * time.Second

	// A refusal is only half the answer. How long it lasts is what decides whether
	// a limiter should slow down or stop, so the probe waits it out and times it.
	probeRecoveryPoll = 10 * time.Second
	probeRecoveryMax  = 6 * time.Minute
)

// probeConcurrencies is the ladder. Cumulative counts and elapsed time are
// carried across rungs, so a limit expressed as "N requests then no" and one
// expressed as "N per second" can be told apart at the end.
var probeConcurrencies = []int{1, 2, 3, 5, 8, 12}

// probeEndpoint is one measurable Steam surface.
type probeEndpoint struct {
	name string
	// urls resolves the request targets once, before the ladder starts. Avatars
	// are not addressable from an account id alone, so this may itself make a
	// small number of requests; they are not counted against the budget because
	// they go to a different service from the one being measured.
	urls func(t *testing.T, ids []string) []string
	// cacheKeyed reports whether a nonce in the query string reaches origin. Where
	// it does not, the ladder would measure the edge instead, and says so.
	cacheKeyed bool
	// freshWindow is the Cache-Control lifetime, used to tell an origin response
	// from a replayed one. Zero where the endpoint does not publish one.
	freshWindow time.Duration
}

var probeEndpoints = map[string]probeEndpoint{
	// The profile XML the refresh reads for ban state, persona and avatar URL.
	"profile": {
		name:        "profile-xml",
		cacheKeyed:  true,
		freshWindow: time.Hour,
		urls: func(t *testing.T, ids []string) []string {
			out := make([]string, 0, len(ids))
			for _, id := range ids {
				out = append(out, fmt.Sprintf("https://steamcommunity.com/profiles/%s?xml=1", id))
			}
			return out
		},
	},
	// The miniprofile fragment. This is the endpoint that refused after roughly
	// twenty requests, and the one the refresh calls once per account - so its
	// ceiling is the one most likely to be met in ordinary use.
	"miniprofile": {
		name:       "miniprofile",
		cacheKeyed: true,
		urls: func(t *testing.T, ids []string) []string {
			out := make([]string, 0, len(ids))
			for _, id := range ids {
				formats, err := FormatsFromID64(id)
				if err != nil {
					t.Fatalf("steam id %s: %v", id, err)
				}
				out = append(out, fmt.Sprintf("https://steamcommunity.com/miniprofile/%s", formats.ID32))
			}
			return out
		},
	},
	// The avatar CDN, which carries the largest share of a refresh by request
	// count and by far the largest by bytes. Only the static avatar is probed:
	// nameplate and animated avatar media run to megabytes each, and measuring a
	// limit is not worth pulling that much of it.
	"avatar": {
		name:       "avatar-cdn",
		cacheKeyed: true,
		urls:       avatarURLs,
	},
}

// avatarURLs resolves each account's full-size avatar by reading its profile XML
// once. Those reads go to the community host, not to the CDN under test.
func avatarURLs(t *testing.T, ids []string) []string {
	t.Helper()
	client := &http.Client{Timeout: probeRequestBudget}
	var out []string
	for _, id := range ids {
		url := fmt.Sprintf("https://steamcommunity.com/profiles/%s?xml=1", id)
		request, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("User-Agent", "TcNo Account Switcher")
		response, err := client.Do(request)
		if err != nil {
			t.Fatalf("resolve avatar for %s: %v", id, err)
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		response.Body.Close()
		if readErr != nil {
			t.Fatalf("resolve avatar for %s: %v", id, readErr)
		}
		doc, err := parseProfileXMLDoc(body)
		if err != nil {
			t.Fatalf("resolve avatar for %s: %v", id, err)
		}
		if avatar := strings.TrimSpace(doc.AvatarFull); avatar != "" {
			out = append(out, avatar)
		}
	}
	if len(out) < 2 {
		t.Skipf("resolved only %d avatar URLs; need at least two", len(out))
	}
	return out
}

type probeOutcome struct {
	concurrency int
	requests    int
	ok          int
	origin      int
	elapsed     time.Duration
	latencies   []time.Duration
	statuses    map[int]int
	refusal     *probeRefusal
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
	return sorted[int(float64(len(sorted)-1)*p)]
}

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

// isRefusal separates "Steam declined to serve this" from "that profile does not
// exist". A 404 is an answer about an account; the rest are answers about us.
//
// 500 and 403 are in the list because that is what the community endpoints
// actually say. Neither is a 429, and a probe that waited for a 429 would report
// a blocked endpoint as healthy.
func isRefusal(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusForbidden, http.StatusUnauthorized,
		http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
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

func probeCacheAllowed() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("TCNO_PROBE_CACHE")), "allow")
}

// bustCache appends a nonce so the edge has nothing to replay. Without it the
// ladder measures Akamai's throughput for a handful of cached objects, which on
// the profile endpoint read as 456 requests a second.
func bustCache(url string, sequence int, at time.Time) string {
	if probeCacheAllowed() {
		return url
	}
	separator := "?"
	if strings.Contains(url, "?") {
		separator = "&"
	}
	return fmt.Sprintf("%s%stcnoprobe=%d-%d", url, separator, at.UnixNano(), sequence)
}

// servedFromOrigin reports whether Steam generated this response rather than the
// edge replaying one. A cached copy has already spent part of its lifetime, so
// its remaining freshness falls short of the full window.
func servedFromOrigin(header http.Header, window time.Duration) (known, origin bool) {
	if window <= 0 {
		return false, false
	}
	date, err := http.ParseTime(header.Get("Date"))
	if err != nil {
		return false, false
	}
	expires, err := http.ParseTime(header.Get("Expires"))
	if err != nil {
		return false, false
	}
	// A second of slack: both are whole seconds and can straddle a tick.
	return true, expires.Sub(date) >= window-time.Second
}

func probeClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	widest := probeConcurrencies[len(probeConcurrencies)-1]
	transport.MaxIdleConns = widest * 2
	transport.MaxIdleConnsPerHost = widest
	return &http.Client{Transport: transport, Timeout: probeRequestBudget}
}

type probeResponse struct {
	latency time.Duration
	status  int
	origin  bool
	body    []byte
	refusal *probeRefusal
}

// probeBodyLimit is generous on purpose. Reading only a few kilobytes and moving
// on measured the time to first bytes, not the time to have the thing - which on
// the avatar CDN is most of the cost and the whole reason it is being measured.
const probeBodyLimit = 2 << 20

func probeOnce(ctx context.Context, client *http.Client, url string, window time.Duration) probeResponse {
	started := time.Now()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return probeResponse{refusal: &probeRefusal{transport: err}}
	}
	request.Header.Set("User-Agent", "TcNo Account Switcher")
	response, err := client.Do(request)
	if err != nil {
		return probeResponse{latency: time.Since(started), refusal: &probeRefusal{transport: err}}
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, probeBodyLimit))
	latency := time.Since(started)
	_, origin := servedFromOrigin(response.Header, window)

	result := probeResponse{latency: latency, status: response.StatusCode, origin: origin, body: body}
	if !isRefusal(response.StatusCode) {
		return result
	}
	head := strings.Join(strings.Fields(string(body)), " ")
	if len(head) > 160 {
		head = head[:160]
	}
	result.refusal = &probeRefusal{
		status:     response.StatusCode,
		retryAfter: response.Header.Get("Retry-After"),
		bodyHead:   head,
	}
	return result
}

func probeRung(ctx context.Context, client *http.Client, endpoint probeEndpoint, urls []string, concurrency, requests int) probeOutcome {
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
			result := probeOnce(ctx, client, url, endpoint.freshWindow)
			mu.Lock()
			defer mu.Unlock()
			outcome.requests++
			outcome.latencies = append(outcome.latencies, result.latency)
			outcome.statuses[result.status]++
			if result.origin {
				outcome.origin++
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
		}(bustCache(urls[i%len(urls)], i, started))
	}
	wg.Wait()
	outcome.elapsed = time.Since(started)
	return outcome
}

// measureRecovery waits out a refusal one request at a time, and reports how long
// it lasted. This is the number a limiter is actually built around: a block that
// clears in ten seconds is worth backing off through, and one that lasts ten
// minutes is worth never provoking.
func measureRecovery(t *testing.T, client *http.Client, endpoint probeEndpoint, url string) {
	t.Helper()
	t.Logf("measuring how long the refusal lasts, one request every %s (giving up after %s)",
		probeRecoveryPoll, probeRecoveryMax)
	ctx, cancel := context.WithTimeout(context.Background(), probeRecoveryMax)
	defer cancel()
	started := time.Now()
	for attempt := 1; ; attempt++ {
		select {
		case <-ctx.Done():
			t.Logf("still refusing after %s - the block outlasts this probe's patience, "+
				"so treat it as expensive and pace well clear of it", probeRecoveryMax)
			return
		case <-time.After(probeRecoveryPoll):
		}
		result := probeOnce(ctx, client, bustCache(url, attempt, time.Now()), endpoint.freshWindow)
		if result.refusal == nil && result.status >= 200 && result.status < 300 {
			t.Logf("RECOVERED after %s (%d polls)", time.Since(started).Round(time.Second), attempt)
			return
		}
		t.Logf("  still refused after %s (status %d)", time.Since(started).Round(time.Second), result.status)
	}
}

func TestSteamRateLimitProbe(t *testing.T) {
	ids := probeIDs(t)
	name := strings.ToLower(strings.TrimSpace(os.Getenv("TCNO_PROBE_ENDPOINT")))
	if name == "" {
		name = "profile"
	}
	endpoint, ok := probeEndpoints[name]
	if !ok {
		known := make([]string, 0, len(probeEndpoints))
		for key := range probeEndpoints {
			known = append(known, key)
		}
		sort.Strings(known)
		t.Fatalf("TCNO_PROBE_ENDPOINT must be one of %v, got %q", known, name)
	}
	budget := probeMaxRequests
	if raw := strings.TrimSpace(os.Getenv("TCNO_PROBE_MAX")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			t.Fatalf("TCNO_PROBE_MAX must be a positive integer, got %q", raw)
		}
		budget = parsed
	}

	urls := endpoint.urls(t, ids)
	ctx, cancel := context.WithTimeout(context.Background(), probeMaxDuration)
	defer cancel()
	client := probeClient()
	defer client.CloseIdleConnections()

	mode := "cache-busted (origin)"
	if probeCacheAllowed() {
		mode = "cache allowed (edge may answer)"
	}
	t.Logf("endpoint=%s urls=%d %s ladder=%v budget=%d over %s",
		endpoint.name, len(urls), mode, probeConcurrencies, budget, probeMaxDuration)

	spent := 0
	totalElapsed := time.Duration(0)
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

		outcome := probeRung(ctx, client, endpoint, urls, concurrency, requests)
		spent += outcome.requests
		totalElapsed += outcome.elapsed
		t.Logf("concurrency=%-2d requests=%-3d ok=%-3d origin=%-3d rate=%5.1f/s p50=%-8s p95=%-8s status[%s]",
			outcome.concurrency, outcome.requests, outcome.ok, outcome.origin, outcome.rate(),
			outcome.percentile(0.50).Round(time.Millisecond),
			outcome.percentile(0.95).Round(time.Millisecond),
			outcome.statusHistogram())
		if endpoint.freshWindow > 0 && !probeCacheAllowed() && outcome.origin < outcome.requests {
			t.Logf("  WARNING: %d/%d responses came from cache; this rung does not measure Steam",
				outcome.requests-outcome.origin, outcome.requests)
		}
		if other := outcome.requests - outcome.ok; other > 0 && outcome.refusal == nil {
			t.Logf("  NOTE: %d/%d responses were neither 2xx nor a recognised refusal; see the status column",
				other, outcome.requests)
		}

		if outcome.refusal != nil {
			r := outcome.refusal
			if r.transport != nil {
				t.Logf("REFUSED: transport error: %v", r.transport)
			} else {
				t.Logf("REFUSED: HTTP %d retryAfter=%q body=%q", r.status, r.retryAfter, r.bodyHead)
			}
			// Both shapes, because they lead to different fixes. A count means the
			// budget is per burst and the answer is to make fewer requests; a rate
			// means the answer is to spread the same ones further apart.
			t.Logf("  after %d requests total, %s of wall clock, at concurrency %d (%.1f req/s on that rung)",
				spent, totalElapsed.Round(time.Millisecond), concurrency, outcome.rate())
			if lastClean != nil {
				t.Logf("  last clean rung: concurrency=%d at %.1f req/s", lastClean.concurrency, lastClean.rate())
			} else {
				t.Log("  the narrowest rung already refused; this endpoint wants less than one request at a time")
			}
			measureRecovery(t, client, endpoint, urls[0])
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
		t.Logf("no refusal: %d requests over %s, cleared concurrency=%d at %.1f req/s",
			spent, totalElapsed.Round(time.Millisecond), lastClean.concurrency, lastClean.rate())
		t.Log("this is 'not right now', not 'no limit': Steam publishes none, and community " +
			"throttling varies by edge and by time of day")
	}
}
