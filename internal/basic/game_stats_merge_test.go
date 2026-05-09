package basic

import (
	"encoding/json"
	"testing"
)

func TestMergeGameStatsCustom(t *testing.T) {
	t.Parallel()
	cfg := gameStatsFile{
		StatsDefinitions: map[string]gameDefinition{
			"A": {UniqueID: "a1"},
			"B": {UniqueID: "b1"},
		},
		PlatformCompatibilities: map[string][]string{
			"X": {"p1"},
		},
	}
	custom := []byte(`{
		"StatsDefinitions": {
			"B": {"UniqueId": "b2"},
			"C": {"UniqueId": "c1"}
		},
		"PlatformCompatibilities": {
			"Y": ["p2"]
		}
	}`)
	if err := mergeGameStatsCustom(&cfg, custom); err != nil {
		t.Fatal(err)
	}
	if cfg.StatsDefinitions["A"].UniqueID != "a1" {
		t.Fatalf("A: %+v", cfg.StatsDefinitions["A"])
	}
	if cfg.StatsDefinitions["B"].UniqueID != "b2" {
		t.Fatalf("B: %+v", cfg.StatsDefinitions["B"])
	}
	if cfg.StatsDefinitions["C"].UniqueID != "c1" {
		t.Fatalf("C: %+v", cfg.StatsDefinitions["C"])
	}
	if len(cfg.PlatformCompatibilities) != 2 {
		t.Fatalf("compat keys: %v", cfg.PlatformCompatibilities)
	}
	if cfg.PlatformCompatibilities["X"][0] != "p1" || cfg.PlatformCompatibilities["Y"][0] != "p2" {
		t.Fatalf("compat: %+v", cfg.PlatformCompatibilities)
	}
	_, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
}
