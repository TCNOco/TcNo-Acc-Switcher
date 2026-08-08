package basic

import (
	"context"
	"encoding/json"

	"testing"
)

func TestRegisterAndResolveGameStatsCollector(t *testing.T) {
	const name = "test-collector"
	t.Cleanup(func() { RegisterGameStatsCollector(name, nil) })

	if got := gameStatsCollectorFor(name); got != nil {
		t.Fatal("collector resolved before registration")
	}
	RegisterGameStatsCollector(name, func(context.Context, string) ([]byte, error) {
		return []byte(`{"premier":1}`), nil
	})
	if got := gameStatsCollectorFor(name); got == nil {
		t.Fatal("collector did not resolve after registration")
	}
	RegisterGameStatsCollector(name, nil)
	if got := gameStatsCollectorFor(name); got != nil {
		t.Fatal("collector survived removal")
	}
}

func TestGameStatsCollectorForIgnoresBlankNames(t *testing.T) {
	if got := gameStatsCollectorFor("   "); got != nil {
		t.Fatal("a blank name resolved a collector")
	}
	RegisterGameStatsCollector("  ", func(context.Context, string) ([]byte, error) { return nil, nil })
	if got := gameStatsCollectorFor(""); got != nil {
		t.Fatal("a blank name was registered")
	}
}

func TestFetchAndParseGameStatsUsesTheCollectorInsteadOfTheURL(t *testing.T) {
	def := gameDefinition{
		Fetch: "unit-collector",
		Collect: map[string]collectInstruction{
			"Premiere": {Source: "json", Path: "premier", DisplayAs: "%x%"},
		},
	}
	var gotAccount string
	fetch := func(_ context.Context, accountID string) ([]byte, error) {
		gotAccount = accountID
		return []byte(`{"premier":15234}`), nil
	}

	// A URL that would fail loudly if it were ever requested.
	_, collected, err := fetchAndParseGameStats(fetch, "http://127.0.0.1:1/should-not-be-fetched", "", "Steam", "Counter-Strike 2", "acct", def)
	if err != nil {
		t.Fatalf("fetchAndParseGameStats: %v", err)
	}
	if gotAccount != "acct" {
		t.Fatalf("collector saw accountID %q", gotAccount)
	}
	if collected["Premiere"] != "15234" {
		t.Fatalf("collected = %#v", collected)
	}
}

func TestFetchAndParseGameStatsFailsWhenTheNamedCollectorIsMissing(t *testing.T) {
	// Falling through to the (usually empty) Url would surface as a confusing
	// transport error rather than the real cause.
	def := gameDefinition{Fetch: "never-registered"}
	_, _, err := fetchAndParseGameStats(nil, "", "", "Steam", "Counter-Strike 2", "acct", def)
	if err == nil {
		t.Fatal("a variant naming an unregistered collector succeeded")
	}
}

func TestCollectorErrorFailsOnlyThatVariant(t *testing.T) {
	// This is the whole "authenticated if we have it, otherwise the public API"
	// mechanism: the collector reports no data, and the chain moves on.
	variants := []gameStatsVariant{
		{
			def:   gameDefinition{Fetch: "unit-collector", Collect: map[string]collectInstruction{"Premiere": {Source: "json", Path: "premier", DisplayAs: "%x%"}}},
			fetch: func(context.Context, string) ([]byte, error) { return nil, ErrGameStatsCollectorNoData },
		},
		{
			def: gameDefinition{Collect: map[string]collectInstruction{"Premiere": {Source: "json", Path: "premier", DisplayAs: "%x%"}}},
			fetch: func(context.Context, string) ([]byte, error) {
				return []byte(`{"premier":900}`), nil
			},
		},
	}
	res := attemptGameStatsChain("Steam", "Counter-Strike 2", "acct", variants, 0)
	if res.index != 1 {
		t.Fatalf("chain settled on variant %d, want the fallback", res.index)
	}
	if res.collected["Premiere"] != "900" {
		t.Fatalf("collected = %#v", res.collected)
	}
	// A missing authenticated source must not look like "this account has no
	// stats anywhere", which is what drops the row.
	if res.allNotFound {
		t.Fatal("allNotFound = true; a collector miss would disable the row")
	}
}

func TestFetchIsNotInheritedByFallbacks(t *testing.T) {
	// Fallbacks inherit everything they do not restate. If Fetch were inherited,
	// every fallback would point at the same collector and the chain would
	// collapse to one source.
	raw := json.RawMessage(`{
      "UniqueId":"CSGO",
      "Fetch":"unit-collector",
      "Url":"",
      "Fallbacks":[{"Url":"https://example.invalid/a"}]
    }`)
	var def gameDefinition
	if err := json.Unmarshal(raw, &def); err != nil {
		t.Fatal(err)
	}
	def.raw = raw
	if err := def.resolveFallbacks(); err != nil {
		t.Fatalf("resolveFallbacks: %v", err)
	}
	if def.variantCount() != 2 {
		t.Fatalf("variantCount = %d", def.variantCount())
	}
	if got := def.variantAt(0).Fetch; got != "unit-collector" {
		t.Fatalf("variant 0 Fetch = %q", got)
	}
	if got := def.variantAt(1).Fetch; got != "" {
		t.Fatalf("variant 1 inherited Fetch = %q, want empty", got)
	}
}
