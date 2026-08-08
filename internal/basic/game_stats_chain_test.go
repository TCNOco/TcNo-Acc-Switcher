package basic

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// loadShippedCS2Definition returns the Counter-Strike 2 definition from the real
// GameStats.json, normalized and with its Fallbacks resolved, exactly as at runtime.
func loadShippedCS2Definition(t *testing.T) gameDefinition {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "GameStats.json"))
	if err != nil {
		t.Skipf("GameStats.json unavailable: %v", err)
	}
	var cfg gameStatsFile
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	def, ok := cfg.StatsDefinitions["Counter-Strike 2"]
	if !ok {
		t.Fatal("Counter-Strike 2 definition missing")
	}
	normalizeGameDefinition(&def)
	if err := def.resolveFallbacks(); err != nil {
		t.Fatal(err)
	}
	for i := range def.resolved {
		normalizeGameDefinition(&def.resolved[i])
	}
	return def
}

// jsonStatVariant builds a variant whose single metric reads "value" out of a JSON body.
func jsonStatVariant(url string) gameStatsVariant {
	def := gameDefinition{
		Collect: map[string]collectInstruction{
			"Rank": {Source: "json", Path: "value", DisplayAs: "%x%"},
		},
	}
	return gameStatsVariant{def: def, url: url}
}

// statusServer replies with a fixed status and body, counting the requests it received.
func statusServer(t *testing.T, status int, body string, hits *int32) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(hits, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestChainFallsThroughToNextSourceOn404(t *testing.T) {
	var primaryHits, fallbackHits int32
	primary := statusServer(t, http.StatusNotFound, `{"StatusCode":404}`, &primaryHits)
	fallback := statusServer(t, http.StatusOK, `{"value":18}`, &fallbackHits)

	variants := []gameStatsVariant{jsonStatVariant(primary), jsonStatVariant(fallback)}
	res := attemptGameStatsChain("Steam", "Counter-Strike 2", "acct", variants, 0)

	if res.index != 1 {
		t.Fatalf("expected the fallback to win, got index %d (err %v)", res.index, res.err)
	}
	if res.collected["Rank"] != "18" {
		t.Fatalf("collected: %+v", res.collected)
	}
	if res.err != nil {
		t.Fatalf("winning chain should clear the error: %v", res.err)
	}
	if res.allNotFound {
		t.Fatal("a successful chain must never report allNotFound")
	}
	if primaryHits != 1 || fallbackHits != 1 {
		t.Fatalf("hits: primary=%d fallback=%d", primaryHits, fallbackHits)
	}
}

func TestChainStartsAtRememberedIndex(t *testing.T) {
	var primaryHits, fallbackHits int32
	primary := statusServer(t, http.StatusOK, `{"value":1}`, &primaryHits)
	fallback := statusServer(t, http.StatusOK, `{"value":2}`, &fallbackHits)

	variants := []gameStatsVariant{jsonStatVariant(primary), jsonStatVariant(fallback)}
	res := attemptGameStatsChain("Steam", "Counter-Strike 2", "acct", variants, 1)

	if res.index != 1 || res.collected["Rank"] != "2" {
		t.Fatalf("remembered index should be tried first: index=%d collected=%+v", res.index, res.collected)
	}
	if primaryHits != 0 {
		t.Fatalf("the known-good source should be the only one contacted, primary hit %d times", primaryHits)
	}
}

// The remembered source going bad must not strand the account on it.
func TestChainRecoversWhenRememberedSourceFails(t *testing.T) {
	var primaryHits, fallbackHits int32
	primary := statusServer(t, http.StatusOK, `{"value":7}`, &primaryHits)
	fallback := statusServer(t, http.StatusNotFound, `{"StatusCode":404}`, &fallbackHits)

	variants := []gameStatsVariant{jsonStatVariant(primary), jsonStatVariant(fallback)}
	res := attemptGameStatsChain("Steam", "Counter-Strike 2", "acct", variants, 1)

	if res.index != 0 || res.collected["Rank"] != "7" {
		t.Fatalf("should fall back to the primary: index=%d collected=%+v", res.index, res.collected)
	}
	if fallbackHits != 1 || primaryHits != 1 {
		t.Fatalf("hits: primary=%d fallback=%d", primaryHits, fallbackHits)
	}
}

func TestChainReportsAllNotFoundOnlyWhenEverySourceIsMissing(t *testing.T) {
	var aHits, bHits int32
	a := statusServer(t, http.StatusNotFound, `{"StatusCode":404}`, &aHits)
	b := statusServer(t, http.StatusGone, `{"StatusCode":410}`, &bHits)

	res := attemptGameStatsChain("Steam", "Counter-Strike 2", "acct",
		[]gameStatsVariant{jsonStatVariant(a), jsonStatVariant(b)}, 0)

	if res.index != -1 {
		t.Fatalf("nothing should have succeeded, got index %d", res.index)
	}
	if !res.allNotFound {
		t.Fatal("404 + 410 across the whole chain should report allNotFound")
	}
	if res.err == nil {
		t.Fatal("exhausted chain must report an error")
	}
}

// A transient failure alongside a genuine 404 must not disable the account: only an
// all-not-found chain is allowed to drop the stats row.
func TestChainDoesNotReportAllNotFoundWhenASourceErrorsOtherwise(t *testing.T) {
	var aHits, bHits int32
	a := statusServer(t, http.StatusInternalServerError, `{}`, &aHits)
	b := statusServer(t, http.StatusNotFound, `{"StatusCode":404}`, &bHits)

	res := attemptGameStatsChain("Steam", "Counter-Strike 2", "acct",
		[]gameStatsVariant{jsonStatVariant(a), jsonStatVariant(b)}, 0)

	if res.index != -1 {
		t.Fatalf("nothing should have succeeded, got index %d", res.index)
	}
	if res.allNotFound {
		t.Fatal("a 500 in the chain must prevent the account's stats row being dropped")
	}
}

// A source that answers 200 but yields no metrics should hand over to the next one.
func TestChainTreatsEmptyResultAsFailureAndKeepsTheBodyForDebugging(t *testing.T) {
	var emptyHits, goodHits int32
	empty := statusServer(t, http.StatusOK, `{"unrelated":1}`, &emptyHits)
	good := statusServer(t, http.StatusOK, `{"value":12}`, &goodHits)

	res := attemptGameStatsChain("Steam", "Counter-Strike 2", "acct",
		[]gameStatsVariant{jsonStatVariant(empty), jsonStatVariant(good)}, 0)

	if res.index != 1 || res.collected["Rank"] != "12" {
		t.Fatalf("should advance past the empty source: index=%d collected=%+v", res.index, res.collected)
	}

	// When nothing works, the empty body is retained so the debug dump still happens.
	only := attemptGameStatsChain("Steam", "Counter-Strike 2", "acct",
		[]gameStatsVariant{jsonStatVariant(empty)}, 0)
	if only.index != -1 || !only.hadEmpty {
		t.Fatalf("expected an exhausted chain flagged empty: index=%d hadEmpty=%v", only.index, only.hadEmpty)
	}
	if len(only.emptyRaw) == 0 {
		t.Fatal("empty body should be kept for the debug dump")
	}
	if only.allNotFound {
		t.Fatal("an empty 200 is not a not-found")
	}
}

// End-to-end over the shipped config: the CS2 fallback must actually parse the payload
// the tcno-api csrepStats endpoint returns, through the inherited display markup.
func TestShippedCS2FallbackParsesCsrepPayload(t *testing.T) {
	def := loadShippedCS2Definition(t)
	// Variant 0 is the authenticated GCPD collector; 1 and 2 are the public APIs.
	if def.variantCount() != 3 {
		t.Fatalf("expected the CS2 chain to be GCPD, Leetify and CSRep, got %d variants", def.variantCount())
	}

	// Leetify stands in answering 404 for an account that never signed up.
	var leetifyHits, csrepHits int32
	leetify := statusServer(t, http.StatusNotFound, `{"StatusCode":404}`, &leetifyHits)
	csrep := statusServer(t, http.StatusOK, `{"premier":15234,"compRank":18}`, &csrepHits)
	res := attemptGameStatsChain("Steam", "Counter-Strike 2", "acct", []gameStatsVariant{
		{def: def.variantAt(1), url: leetify},
		{def: def.variantAt(2), url: csrep},
	}, 0)

	if res.index != 1 {
		t.Fatalf("fallback should have parsed the payload: index=%d err=%v", res.index, res.err)
	}
	premier, ok := res.collected["Premiere"]
	if !ok {
		t.Fatalf("no Premiere metric collected: %+v", res.collected)
	}
	if !strings.Contains(premier, "15,234") {
		t.Fatalf("premier should render through the inherited commaNumber format: %s", premier)
	}
	// 15234 lands in the 15000-19999 DisplayPlaceholders band.
	if !strings.Contains(premier, "#bc6bfd") {
		t.Fatalf("premier should pick the inherited colour band: %s", premier)
	}
	comp, ok := res.collected["CompRank"]
	if !ok {
		t.Fatalf("no CompRank metric collected: %+v", res.collected)
	}
	if !strings.Contains(comp, "comp18.webp") {
		t.Fatalf("comp rank should render the inherited rank image: %s", comp)
	}
}

// csrepStats reports an absent metric as 0, which NoDisplayIf must hide while still
// showing the metric that does have data.
func TestShippedCS2FallbackHidesZeroedMetric(t *testing.T) {
	def := loadShippedCS2Definition(t)

	var hits int32
	url := statusServer(t, http.StatusOK, `{"premier":0,"compRank":16}`, &hits)
	res := attemptGameStatsChain("Steam", "Counter-Strike 2", "acct",
		[]gameStatsVariant{{def: def.variantAt(2), url: url}}, 0)

	if res.index != 0 {
		t.Fatalf("the only source should have succeeded: index=%d err=%v", res.index, res.err)
	}
	if _, ok := res.collected["Premiere"]; ok {
		t.Fatalf("a zero premier rating should be hidden: %+v", res.collected)
	}
	if comp := res.collected["CompRank"]; !strings.Contains(comp, "comp16.webp") {
		t.Fatalf("comp rank should still render: %q", comp)
	}
}

func TestChainSucceedsOnPrimaryWithoutContactingFallback(t *testing.T) {
	var primaryHits, fallbackHits int32
	primary := statusServer(t, http.StatusOK, `{"value":5}`, &primaryHits)
	fallback := statusServer(t, http.StatusOK, `{"value":9}`, &fallbackHits)

	res := attemptGameStatsChain("Steam", "Counter-Strike 2", "acct",
		[]gameStatsVariant{jsonStatVariant(primary), jsonStatVariant(fallback)}, 0)

	if res.index != 0 || res.collected["Rank"] != "5" {
		t.Fatalf("primary should win: index=%d collected=%+v", res.index, res.collected)
	}
	if fallbackHits != 0 {
		t.Fatalf("fallback should not be contacted when the primary works, hit %d times", fallbackHits)
	}
}
