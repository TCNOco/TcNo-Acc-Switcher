package basic

import (
	"strings"
	"testing"
)

func TestApplyVariableTransformPipeline_ReplaceLast_BattleTagStyle(t *testing.T) {
	s := "YTTechNobo_1234|ReplaceLast('_','-')"
	got := applyVariableTransformPipeline(s)
	if got != "YTTechNobo-1234" {
		t.Fatalf("got %q want YTTechNobo-1234", got)
	}
}

func TestApplyVariableTransformPipeline_ReplaceFirst(t *testing.T) {
	got := applyVariableTransformPipeline(`abxab|ReplaceFirst('ab','Z')`)
	if got != "Zxab" {
		t.Fatalf("got %q want Zxab", got)
	}
}

func TestApplyVariableTransformPipeline_ReplaceAll(t *testing.T) {
	got := applyVariableTransformPipeline(`aa|ReplaceAll('a','b')`)
	if got != "bb" {
		t.Fatalf("got %q want bb", got)
	}
}

func TestApplyVariableTransformPipeline_Substring(t *testing.T) {
	if g := applyVariableTransformPipeline(`abcdef|Substring(1:4)`); g != "bcd" {
		t.Fatalf("1:4 got %q", g)
	}
	if g := applyVariableTransformPipeline(`abcdef|Substring(2:)`); g != "cdef" {
		t.Fatalf("2: got %q", g)
	}
	if g := applyVariableTransformPipeline(`abcdef|Substring(:3)`); g != "abc" {
		t.Fatalf(":3 got %q", g)
	}
	// Out of range: tolerant
	if g := applyVariableTransformPipeline(`ab|Substring(5:8)`); g != "" {
		t.Fatalf("expected empty, got %q", g)
	}
}

func TestApplyVariableTransformPipeline_Chained(t *testing.T) {
	got := applyVariableTransformPipeline(`a_b_c|ReplaceLast('_','-')|ReplaceAll('a','@')`)
	if got != "@_b-c" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveGameStatsVarTemplates_BattleTagFromAccountUsername(t *testing.T) {
	def := map[string]string{
		"BattleTag": "%ACCOUNTUSERNAME%|ReplaceLast('_','-')",
	}
	ctx := GameStatVarContext{AccountID: "1", AccountUsername: "YTTechNobo_1234"}
	got := ResolveGameStatsVarTemplates(def, map[string]string{}, ctx)
	if got["BattleTag"] != "YTTechNobo-1234" {
		t.Fatalf("got %q", got["BattleTag"])
	}
}

func TestResolveGameStatsVarTemplates_UsernameAliasNoFallback(t *testing.T) {
	def := map[string]string{
		"Username": "%Username%",
	}
	ctx := GameStatVarContext{
		AccountID:       "76561197960287930",
		AccountUsername: "76561197960287930",
		Username:        "",
	}
	got := ResolveGameStatsVarTemplates(def, map[string]string{}, ctx)
	if got["Username"] != "" {
		t.Fatalf("expected empty Username alias when raw username missing, got %q", got["Username"])
	}
}

func TestResolveGameStatsVarTemplates_PlainTokenPrefersStored(t *testing.T) {
	t.Parallel()
	def := map[string]string{"Username": "%Username%"}
	ctx := GameStatVarContext{AccountID: "1", AccountUsername: "steam", Username: "ea_default"}
	got := ResolveGameStatsVarTemplates(def, map[string]string{"Username": "MyEAName"}, ctx)
	if got["Username"] != "MyEAName" {
		t.Fatalf("stored EA username should win over token, got %q", got["Username"])
	}
}

func TestResolveGameStatsVarTemplates_PlainAccountIDToken(t *testing.T) {
	t.Parallel()
	def := map[string]string{"SteamId": "%ACCOUNTID%"}
	ctx := GameStatVarContext{AccountID: "76561197960287930", AccountUsername: "x"}
	got := ResolveGameStatsVarTemplates(def, map[string]string{}, ctx)
	if got["SteamId"] != "76561197960287930" {
		t.Fatalf("got %q", got["SteamId"])
	}
}

func TestResolveGameStatsVarTemplates_DependencyChain(t *testing.T) {
	def := map[string]string{
		"B":     "B field",
		"A":     "%B%|ReplaceAll('e','3')",
		"Outer": "%A%|ReplaceAll('t','T')",
	}
	stored := map[string]string{"B": "letter"}
	ctx := GameStatVarContext{AccountID: "x", AccountUsername: "u"}
	got := ResolveGameStatsVarTemplates(def, stored, ctx)
	if got["B"] != "letter" {
		t.Fatalf("B got %q", got["B"])
	}
	if got["A"] != "l3tt3r" {
		t.Fatalf("A got %q", got["A"])
	}
	if got["Outer"] != "l3TT3r" {
		t.Fatalf("Outer got %q", got["Outer"])
	}
}

func TestResolveGameStatsVarTemplates_CyclicValuesResolveEmpty(t *testing.T) {
	def := map[string]string{
		"A": "%B%",
		"B": "%A%",
	}
	stored := map[string]string{}
	ctx := GameStatVarContext{AccountID: "1", AccountUsername: "u"}
	got := ResolveGameStatsVarTemplates(def, stored, ctx)
	if got["A"] != "" || got["B"] != "" {
		t.Fatalf("cyclic vars should resolve empty, got %#v", got)
	}
}

func TestTopoSortGameStatVars_OrderAndCycleFallback(t *testing.T) {
	keys := []string{"A", "B", "C"}
	def := map[string]string{
		"C": "%B%",
		"B": "%A%",
		"A": "plain",
	}
	order, ok := topoSortGameStatVars(keys, def)
	if !ok || len(order) != 3 {
		t.Fatalf("expected acyclic sort, ok=%v order=%v", ok, order)
	}
	// A must come before B and C; B before C
	idx := func(s string) int {
		for i, x := range order {
			if x == s {
				return i
			}
		}
		return -1
	}
	if idx("A") > idx("B") || idx("B") > idx("C") {
		t.Fatalf("bad topo order: %v", order)
	}

	cycleKeys := []string{"X", "Y"}
	cycleDef := map[string]string{
		"X": "%Y%",
		"Y": "%X%",
	}
	order2, ok2 := topoSortGameStatVars(cycleKeys, cycleDef)
	if ok2 {
		t.Fatalf("expected cycle detection")
	}
	if len(order2) != 2 || !strings.Contains(strings.Join(order2, ","), "X") {
		t.Fatalf("fallback order unexpected: %v", order2)
	}
}
