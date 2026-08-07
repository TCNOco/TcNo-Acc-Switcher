package basic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func mergedString(t *testing.T, base, override string) map[string]any {
	t.Helper()
	merged, err := deepMergeJSON(json.RawMessage(base), json.RawMessage(override))
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(merged, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestDeepMergeJSONObjectsMergeByKey(t *testing.T) {
	t.Parallel()
	got := mergedString(t,
		`{"Url":"a","Collect":{"One":{"Path":"p1","DisplayAs":"keep"},"Two":{"Path":"p2"}}}`,
		`{"Collect":{"One":{"Path":"changed"}}}`)

	if got["Url"] != "a" {
		t.Fatalf("unspecified key should survive: %v", got["Url"])
	}
	collect := got["Collect"].(map[string]any)
	one := collect["One"].(map[string]any)
	if one["Path"] != "changed" {
		t.Fatalf("override should win: %v", one["Path"])
	}
	if one["DisplayAs"] != "keep" {
		t.Fatalf("sibling key inside a nested object should be inherited: %v", one["DisplayAs"])
	}
	if _, ok := collect["Two"]; !ok {
		t.Fatal("untouched nested entry should survive")
	}
}

func TestDeepMergeJSONArraysAndScalarsReplace(t *testing.T) {
	t.Parallel()
	got := mergedString(t,
		`{"Pipeline":["a","b","c"],"Reducer":"maxNumber","TTL":30}`,
		`{"Pipeline":["z"],"Reducer":"","TTL":60}`)

	pipeline := got["Pipeline"].([]any)
	if len(pipeline) != 1 || pipeline[0] != "z" {
		t.Fatalf("arrays must replace, not concatenate: %v", pipeline)
	}
	if got["Reducer"] != "" {
		t.Fatalf("empty string must override: %q", got["Reducer"])
	}
	if got["TTL"].(float64) != 60 {
		t.Fatalf("scalar override: %v", got["TTL"])
	}
}

func TestDeepMergeJSONNullClearsInheritedValue(t *testing.T) {
	t.Parallel()
	got := mergedString(t,
		`{"Attribution":{"Image":"leetify.webp","Dimensions":"270x115","Link":"https://leetify.com"}}`,
		`{"Attribution":{"Image":null,"Dimensions":null,"Text":"CSRep.gg","Link":"https://csrep.gg"}}`)

	attr := got["Attribution"].(map[string]any)
	if attr["Image"] != nil || attr["Dimensions"] != nil {
		t.Fatalf("null must clear the parent value, got Image=%v Dimensions=%v", attr["Image"], attr["Dimensions"])
	}
	if attr["Text"] != "CSRep.gg" || attr["Link"] != "https://csrep.gg" {
		t.Fatalf("attribution override: %v", attr)
	}
}

func TestDeepMergeJSONKindMismatchOverrideWins(t *testing.T) {
	t.Parallel()
	merged, err := deepMergeJSON(json.RawMessage(`{"a":1}`), json.RawMessage(`"scalar"`))
	if err != nil {
		t.Fatal(err)
	}
	if string(merged) != `"scalar"` {
		t.Fatalf("override should win wholesale: %s", merged)
	}
}

// A fallback should be able to restate only what differs and inherit the rest, including
// display config nested one level inside Collect.
func TestResolveFallbacksInheritsUnspecifiedFields(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
		"UniqueId": "CSGO",
		"ProcessName": "cs2.exe",
		"Url": "https://primary.example/{SteamId}",
		"Attribution": {"Image": "leetify.webp", "Dimensions": "270x115", "Link": "https://leetify.com"},
		"Vars": {"SteamId": {"Autofill": "%ACCOUNTID%"}},
		"Collect": {
			"Premiere": {
				"Source": "json",
				"Path": "ranks.premier",
				"Reducer": "firstMatchInArray",
				"ReducerOptions": {"arrayPath": "recent_matches"},
				"DisplayAs": "<div>%x_fmt%</div>",
				"NoDisplayIf": "0",
				"ToggleText": "Premier Rank"
			}
		},
		"Fallbacks": [
			{
				"Url": "https://fallback.example?steamid={SteamId}",
				"Attribution": {"Image": null, "Dimensions": null, "Text": "CSRep.gg", "Link": "https://csrep.gg"},
				"Collect": {"Premiere": {"Path": "premier", "Reducer": "", "ReducerOptions": null}}
			}
		]
	}`)

	var def gameDefinition
	if err := json.Unmarshal(raw, &def); err != nil {
		t.Fatal(err)
	}
	normalizeGameDefinition(&def)
	if err := def.resolveFallbacks(); err != nil {
		t.Fatal(err)
	}
	for i := range def.resolved {
		normalizeGameDefinition(&def.resolved[i])
	}

	if def.variantCount() != 2 {
		t.Fatalf("variant count: %d", def.variantCount())
	}
	fb := def.variantAt(1)

	if fb.URL != "https://fallback.example?steamid={SteamId}" {
		t.Fatalf("url override: %q", fb.URL)
	}
	if fb.UniqueID != "CSGO" || fb.ProcessName != "cs2.exe" {
		t.Fatalf("top-level fields should be inherited: %+v", fb)
	}
	if fb.Vars["SteamId"].Autofill != "%ACCOUNTID%" {
		t.Fatalf("vars should be inherited: %+v", fb.Vars)
	}
	if fb.Attribution == nil || fb.Attribution.Image != "" || fb.Attribution.Dimensions != "" {
		t.Fatalf("null should clear the inherited logo: %+v", fb.Attribution)
	}
	if fb.Attribution.Text != "CSRep.gg" || fb.Attribution.Link != "https://csrep.gg" {
		t.Fatalf("attribution override: %+v", fb.Attribution)
	}

	prem := fb.Collect["Premiere"]
	if prem.Path != "premier" || prem.Reducer != "" {
		t.Fatalf("collect override: %+v", prem)
	}
	if prem.ReducerOptions != nil {
		t.Fatalf("null should clear reducer options: %+v", prem.ReducerOptions)
	}
	if prem.Source != "json" || prem.DisplayAs != "<div>%x_fmt%</div>" || prem.NoDisplayIf != "0" || prem.ToggleText != "Premier Rank" {
		t.Fatalf("display config should be inherited: %+v", prem)
	}

	// The primary definition must be untouched by resolution.
	if def.Collect["Premiere"].Path != "ranks.premier" || def.Attribution.Image != "leetify.webp" {
		t.Fatalf("primary definition mutated: %+v / %+v", def.Collect["Premiere"], def.Attribution)
	}
	// A merged variant never carries its own chain.
	if len(fb.Fallbacks) != 0 || len(fb.resolved) != 0 {
		t.Fatalf("fallbacks must not nest: %d/%d", len(fb.Fallbacks), len(fb.resolved))
	}
}

// Every source is credited up front, so a fallback is discoverable before it activates.
func TestCollectVariantAttributionsListsEverySourceInTryOrder(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
		"Attribution": {"Image": "leetify.webp", "Dimensions": "270x115", "Link": "https://leetify.com"},
		"Fallbacks": [
			{"Attribution": {"Image": "csrep.webp", "Dimensions": "270x80", "Text": "CSRep.gg", "Link": "https://csrep.gg"}},
			{"Attribution": {"Image": null, "Dimensions": null, "Text": "Third", "Link": "https://third.example"}}
		]
	}`)
	var def gameDefinition
	if err := json.Unmarshal(raw, &def); err != nil {
		t.Fatal(err)
	}
	if err := def.resolveFallbacks(); err != nil {
		t.Fatal(err)
	}

	got := collectVariantAttributions(def)
	if len(got) != 3 {
		t.Fatalf("expected 3 sources, got %d: %+v", len(got), got)
	}
	if got[0].Image != "leetify.webp" || got[0].Link != "https://leetify.com" {
		t.Fatalf("primary must come first: %+v", got[0])
	}
	if got[1].Image != "csrep.webp" || got[1].Dimensions != "270x80" {
		t.Fatalf("first fallback: %+v", got[1])
	}
	if got[2].Image != "" || got[2].Text != "Third" {
		t.Fatalf("second fallback: %+v", got[2])
	}
}

// A fallback that only swaps the URL inherits the primary's credit; listing it twice would
// show the same logo side by side with itself.
func TestCollectVariantAttributionsDedupesInheritedCredit(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
		"Attribution": {"Image": "leetify.webp", "Link": "https://leetify.com"},
		"Fallbacks": [{"Url": "https://mirror.example"}]
	}`)
	var def gameDefinition
	if err := json.Unmarshal(raw, &def); err != nil {
		t.Fatal(err)
	}
	if err := def.resolveFallbacks(); err != nil {
		t.Fatal(err)
	}
	if got := collectVariantAttributions(def); len(got) != 1 {
		t.Fatalf("inherited credit should collapse to one entry, got %d: %+v", len(got), got)
	}
}

func TestCollectVariantAttributionsSkipsEmptyCredit(t *testing.T) {
	t.Parallel()
	var def gameDefinition
	if err := json.Unmarshal([]byte(`{"Url": "https://example.com"}`), &def); err != nil {
		t.Fatal(err)
	}
	if got := collectVariantAttributions(def); len(got) != 0 {
		t.Fatalf("a definition with no attribution should credit nothing: %+v", got)
	}
}

func TestShippedCS2CreditsBothSources(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "GameStats.json"))
	if err != nil {
		t.Skipf("GameStats.json unavailable: %v", err)
	}
	var cfg gameStatsFile
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	def := cfg.StatsDefinitions["Counter-Strike 2"]
	if err := def.resolveFallbacks(); err != nil {
		t.Fatal(err)
	}
	got := collectVariantAttributions(def)
	if len(got) != 2 {
		t.Fatalf("CS2 should credit Leetify and CSRep, got %d: %+v", len(got), got)
	}
	if got[0].Link != "https://leetify.com" || got[1].Link != "https://csrep.gg" {
		t.Fatalf("credits out of order: %+v", got)
	}
	// Both need a parseable size, or the modal cannot cap them against their own width.
	for _, a := range got {
		if a.Image == "" {
			continue
		}
		if !regexp.MustCompile(`^\d+x\d+$`).MatchString(a.Dimensions) {
			t.Fatalf("logo %q needs WxH dimensions, got %q", a.Image, a.Dimensions)
		}
	}
}

func TestVariantAtOutOfRangeFallsBackToPrimary(t *testing.T) {
	t.Parallel()
	def := gameDefinition{URL: "primary", resolved: []gameDefinition{{URL: "fb0"}}}
	if def.variantAt(1).URL != "fb0" {
		t.Fatalf("index 1 should be the first fallback")
	}
	// A cached index that no longer exists must not panic or pick nothing.
	for _, idx := range []int{-1, 0, 2, 99} {
		if got := def.variantAt(idx).URL; got != "primary" {
			t.Fatalf("index %d should clamp to primary, got %q", idx, got)
		}
	}
}

func TestFallbackAttemptOrderTriesRememberedFirstThenTheRest(t *testing.T) {
	t.Parallel()
	cases := []struct {
		start, count int
		want         []int
	}{
		{start: 0, count: 3, want: []int{0, 1, 2}},
		{start: 1, count: 3, want: []int{1, 0, 2}},
		{start: 2, count: 3, want: []int{2, 0, 1}},
		{start: 0, count: 1, want: []int{0}},
		// Remembered index past the end of a shortened chain restarts at the primary.
		{start: 7, count: 2, want: []int{0, 1}},
		{start: 0, count: 0, want: nil},
	}
	for _, c := range cases {
		got := fallbackAttemptOrder(c.start, c.count)
		if len(got) != len(c.want) {
			t.Fatalf("start=%d count=%d: got %v want %v", c.start, c.count, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("start=%d count=%d: got %v want %v", c.start, c.count, got, c.want)
			}
		}
	}
}

// Guards the shipped CS2 fallback: a typo here would only surface at runtime, after
// Leetify had already failed for a user.
func TestShippedCS2FallbackResolves(t *testing.T) {
	t.Parallel()
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
	if def.variantCount() != 2 {
		t.Fatalf("expected one CS2 fallback, got %d variants", def.variantCount())
	}

	fb := def.variantAt(1)
	if fb.URL != "https://api.tcno.co/sw/csrepStats?steamid={SteamId}" {
		t.Fatalf("fallback url: %q", fb.URL)
	}
	// The SteamId var must still resolve, or the URL keeps its placeholder.
	if _, ok := fb.Vars["SteamId"]; !ok {
		t.Fatalf("fallback lost the SteamId var: %+v", fb.Vars)
	}
	if fb.Attribution == nil || fb.Attribution.Link != "https://csrep.gg" || fb.Attribution.Text != "CSRep.gg" {
		t.Fatalf("fallback should credit CSRep, not Leetify: %+v", fb.Attribution)
	}
	if fb.Attribution.Image == def.Attribution.Image {
		t.Fatalf("fallback must not inherit the primary's logo: %q", fb.Attribution.Image)
	}

	prem, ok := fb.Collect["Premiere"]
	if !ok {
		t.Fatal("fallback lost the Premiere metric")
	}
	if prem.Path != "premier" || prem.Source != "json" || prem.NoDisplayIf != "0" {
		t.Fatalf("premiere: %+v", prem)
	}
	if prem.DisplayAs == "" || prem.DisplayAs == "%x%" {
		t.Fatal("premiere should inherit the primary DisplayAs markup")
	}
	comp, ok := fb.Collect["CompRank"]
	if !ok {
		t.Fatal("fallback lost the CompRank metric")
	}
	if comp.Path != "compRank" || comp.Reducer != "" {
		t.Fatalf("comprank: %+v", comp)
	}
	if comp.DisplayAs == "" || comp.DisplayAs == "%x%" {
		t.Fatal("comprank should inherit the primary DisplayAs markup")
	}
}
