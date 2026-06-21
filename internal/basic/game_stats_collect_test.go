package basic

import (
	"errors"
	"strings"
	"testing"
)

func TestIsGameStatsResourceNotFound(t *testing.T) {
	if isGameStatsResourceNotFound(errors.New("HTTP 404")) {
		t.Fatal("plain error string must not match")
	}
	if !isGameStatsResourceNotFound(&GameStatsHTTPError{StatusCode: 404}) {
		t.Fatal("404 should be not-found")
	}
	if !isGameStatsResourceNotFound(&GameStatsHTTPError{StatusCode: 410}) {
		t.Fatal("410 should be not-found")
	}
	if isGameStatsResourceNotFound(&GameStatsHTTPError{StatusCode: 403}) {
		t.Fatal("403 should not disable as not-found")
	}
}

func TestApplySelectFunc_SplitChain(t *testing.T) {
	in := `<img src="/core/level_badge/?level=337" style="width: 85px; height: 85px;" class="center-img">`
	got := applySelectFunc(in, `.split('?level=')[1].split('"')[0]`)
	if got != "337" {
		t.Fatalf("unexpected selectfunc output: got %q want %q", got, "337")
	}
}

func TestApplySelectFunc_OutOfRange(t *testing.T) {
	got := applySelectFunc("abc", `.split('-')[2]`)
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestApplySelectFunc_ChainedCommands_Extensible(t *testing.T) {
	in := `  level=337  `
	got := applySelectFunc(in, `.trim().replace('level=','').split('3')[0].trim()`)
	if got != "" {
		t.Fatalf("unexpected chained output: got %q want empty", got)
	}
	got2 := applySelectFunc(in, `.trim().replace('level=','').toUpper()`)
	if got2 != "337" {
		t.Fatalf("unexpected output: got %q want %q", got2, "337")
	}
}

func TestApplySelectFunc_Substring(t *testing.T) {
	in := "prefix-abcdef"
	got := applySelectFunc(in, `.substring(7:10)`)
	if got != "abc" {
		t.Fatalf("unexpected substring output: got %q want %q", got, "abc")
	}
}

func TestExtractMetricFromJSON_FirstNotNullAndZeroSemantics(t *testing.T) {
	raw := []byte(`{"a":null,"b":0,"c":7}`)
	ci := collectInstruction{
		Source:        "json",
		Path:          "a",
		FallbackPaths: []string{"b", "c"},
		Reducer:       "firstNotNull",
	}
	v, ok := extractMetricFromJSON(raw, ci)
	if !ok || v != "0" {
		t.Fatalf("firstNotNull expected 0, got ok=%v v=%q", ok, v)
	}
	ci.Reducer = "firstNotNullOrZero"
	v, ok = extractMetricFromJSON(raw, ci)
	if !ok || v != "7" {
		t.Fatalf("firstNotNullOrZero expected 7, got ok=%v v=%q", ok, v)
	}
}

func TestExtractMetricFromJSON_MaxNumberArray(t *testing.T) {
	raw := []byte(`{"ranks":{"competitive":[{"rank":4},{"rank":11},{"rank":7}]}}`)
	ci := collectInstruction{
		Source:  "json",
		Path:    "ranks.competitive.#.rank",
		Reducer: "maxNumber",
	}
	v, ok := extractMetricFromJSON(raw, ci)
	if !ok || v != "11" {
		t.Fatalf("maxNumber expected 11, got ok=%v v=%q", ok, v)
	}
}

func TestExtractMetricFromJSON_FirstMatchInArrayFallback(t *testing.T) {
	raw := []byte(`{"ranks":{"premier":null},"recent_matches":[{"rank":null},{"rank":9},{"rank":6}]}`)
	ci := collectInstruction{
		Source:  "json",
		Path:    "ranks.premier",
		Reducer: "firstMatchInArray",
		ReducerOptions: map[string]any{
			"arrayPath":    "recent_matches",
			"valuePath":    "rank",
			"requiredPath": "rank",
		},
	}
	v, ok := extractMetricFromJSON(raw, ci)
	if !ok || v != "9" {
		t.Fatalf("firstMatchInArray expected 9, got ok=%v v=%q", ok, v)
	}
}

func TestExtractMetricFromJSON_FirstMatchInArray_WithPredicates(t *testing.T) {
	raw := []byte(`{
		"recent_matches":[
			{"rank":0,"data_source":"matchmaking"},
			{"rank":12,"data_source":"faceit"},
			{"rank":8,"data_source":"matchmaking"}
		]
	}`)
	ci := collectInstruction{
		Source:  "json",
		Reducer: "firstMatchInArray",
		ReducerOptions: map[string]any{
			"arrayPath":    "recent_matches",
			"valuePath":    "rank",
			"requiredPath": "rank",
			"matchPath":    "data_source",
			"matchEquals":  "matchmaking",
			"valueGt":      "0",
		},
	}
	v, ok := extractMetricFromJSON(raw, ci)
	if !ok || v != "8" {
		t.Fatalf("firstMatchInArray predicates expected 8, got ok=%v v=%q", ok, v)
	}
}

func TestExtractMetricFromJSON_FirstMatchInArray_AllMatch(t *testing.T) {
	raw := []byte(`{
		"recent_matches":[
			{"rank":9,"data_source":"faceit","mode":"premier"},
			{"rank":0,"data_source":"matchmaking","mode":"premier"},
			{"rank":7,"data_source":"matchmaking","mode":"casual"},
			{"rank":12,"data_source":"matchmaking","mode":"premier"}
		]
	}`)
	ci := collectInstruction{
		Source:  "json",
		Reducer: "firstMatchInArray",
		ReducerOptions: map[string]any{
			"arrayPath": "recent_matches",
			"valuePath": "rank",
			"allMatch": []any{
				map[string]any{"path": "data_source", "op": "eq", "value": "matchmaking"},
				map[string]any{"path": "mode", "op": "eq", "value": "premier"},
				map[string]any{"path": "rank", "op": "gt", "value": "0"},
			},
		},
	}
	v, ok := extractMetricFromJSON(raw, ci)
	if !ok || v != "12" {
		t.Fatalf("firstMatchInArray allMatch expected 12, got ok=%v v=%q", ok, v)
	}
}

func f64(v float64) *float64 { return &v }

func TestApplyDisplayPlaceholders_Ranges(t *testing.T) {
	rules := []displayPlaceholderRule{
		{
			Key: "fill",
			Ranges: []displayRangeEntry{
				{Min: f64(0), Max: f64(4999), Value: "#b3cad6"},
				{Min: f64(5000), Max: f64(9999), Value: "#8abce8"},
				{Min: f64(30000), Value: "#fad701"},
			},
			Default: "#def",
		},
	}
	tests := []struct {
		metric string
		want   string
	}{
		{"100", "#b3cad6"},
		{"4999", "#b3cad6"},
		{"5000", "#8abce8"},
		{"9999", "#8abce8"},
		{"10000", "#def"},
		{"30000", "#fad701"},
		{"999999", "#fad701"},
		{"not-a-number", "#def"},
	}
	for _, tt := range tests {
		got := applyDisplayPlaceholders(`fill="%fill%"`, tt.metric, rules)
		want := `fill="` + tt.want + `"`
		if got != want {
			t.Errorf("metric %q: got %q want %q", tt.metric, got, want)
		}
	}
}

func TestApplyDisplayPlaceholders_FirstRangeWins(t *testing.T) {
	rules := []displayPlaceholderRule{
		{
			Key: "c",
			Ranges: []displayRangeEntry{
				{Min: f64(0), Max: f64(10), Value: "a"},
				{Min: f64(0), Max: f64(10), Value: "b"},
			},
			Default: "z",
		},
	}
	got := applyDisplayPlaceholders("%c%", "5", rules)
	if got != "a" {
		t.Fatalf("first matching range should win: got %q", got)
	}
}

func TestApplyDisplayPlaceholders_UnknownFromUsesDefaultToken(t *testing.T) {
	rules := []displayPlaceholderRule{
		{
			Key:     "fill",
			From:    "other",
			Ranges:  []displayRangeEntry{{Min: f64(0), Max: f64(1), Value: "#x"}},
			Default: "#def",
		},
	}
	got := applyDisplayPlaceholders("%fill%", "99", rules)
	if got != "#def" {
		t.Fatalf("non-x From should leave default: got %q", got)
	}
}

func TestApplyDisplayPlaceholders_EmptyRulesNoop(t *testing.T) {
	in := `a%fill%b`
	got := applyDisplayPlaceholders(in, "5000", nil)
	if got != in {
		t.Fatalf("expected noop, got %q", got)
	}
}

func TestApplyDisplayFormat_CommaNumber(t *testing.T) {
	tests := []struct {
		raw, want string
	}{
		{"13000", "13,000"},
		{"7", "7"},
		{"1000", "1,000"},
		{"-2500", "-2,500"},
		{"1234.56", "1,234.56"},
		{"not a number", "not a number"},
		{"", ""},
	}
	for _, tt := range tests {
		got := applyDisplayFormat(tt.raw, "commaNumber")
		if got != tt.want {
			t.Errorf("raw %q: got %q want %q", tt.raw, got, tt.want)
		}
	}
}

func TestCollectStatsFromHTML_RawAndFormattedPlaceholders(t *testing.T) {
	raw := []byte(`{"a":11,"b":13000}`)
	def := gameDefinition{
		Collect: map[string]collectInstruction{
			"M": {
				Source:        "json",
				Path:          "b",
				DisplayFormat: "commaNumber",
				DisplayAs:     `<img src="rank%x%.png"/><span>%x_fmt%</span>`,
			},
		},
	}
	out, err := collectStatsFromHTML("steam", "1", def, nil, raw)
	if err != nil {
		t.Fatal(err)
	}
	s := out["M"]
	if !strings.Contains(s, `src="rank13000.png"`) || !strings.Contains(s, `>13,000<`) {
		t.Fatalf("unexpected output: %q", s)
	}
}

func TestGameDefinitionUsesJSONOnly(t *testing.T) {
	t.Parallel()
	if !gameDefinitionUsesJSONOnly(gameDefinition{
		Collect: map[string]collectInstruction{
			"A": {Source: "json", Path: "a"},
		},
	}) {
		t.Fatal("expected json-only")
	}
	if gameDefinitionUsesJSONOnly(gameDefinition{
		Collect: map[string]collectInstruction{
			"A": {XPath: "//a"},
		},
	}) {
		t.Fatal("expected mixed/html definition")
	}
}

func TestCollectStatsFromHTML_ApexRankDisplay(t *testing.T) {
	raw := []byte(`{"level":123,"rankScore":1234,"rankImg":"https://api.mozambiquehe.re/assets/ranks/platinum4.png"}`)
	def := gameDefinition{
		Collect: map[string]collectInstruction{
			"BR": {
				Source:          "json",
				Path:            "rankScore",
				ImageFromPath:   "rankImg",
				ImageCacheDir:   "gs/apex",
				DisplayFormat:   "commaNumber",
				DisplayAs:       `<div class='apex-rank'><img src="%img%" alt=""/><span>%x_fmt% BR</span></div>`,
			},
		},
	}
	out, err := collectStatsFromHTML("steam", "1", def, nil, raw)
	if err != nil {
		t.Fatal(err)
	}
	s := out["BR"]
	if !strings.Contains(s, `class='apex-rank'`) || !strings.Contains(s, `>1,234 BR<`) {
		t.Fatalf("unexpected BR output: %q", s)
	}
}

func TestCollectStatsFromHTML_JSONWithDisplayPlaceholders(t *testing.T) {
	raw := []byte(`{"v":12000}`)
	def := gameDefinition{
		Collect: map[string]collectInstruction{
			"Premiere": {
				Source:    "json",
				Path:      "v",
				DisplayAs: `<svg fill="%fill%"></svg><span>%x%</span>`,
				DisplayPlaceholders: []displayPlaceholderRule{
					{
						Key: "fill",
						Ranges: []displayRangeEntry{
							{Min: f64(10000), Max: f64(14999), Value: "#667deb"},
						},
						Default: "#000",
					},
				},
			},
		},
	}
	out, err := collectStatsFromHTML("steam", "1", def, nil, raw)
	if err != nil {
		t.Fatal(err)
	}
	s := out["Premiere"]
	if !strings.Contains(s, `fill="#667deb"`) || !strings.Contains(s, ">12000<") {
		t.Fatalf("unexpected collected HTML: %q", s)
	}
}
