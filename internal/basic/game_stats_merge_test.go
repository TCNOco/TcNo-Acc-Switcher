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

func TestGameAttributionJSON(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
		"StatsDefinitions": {
			"Counter-Strike 2": {
				"UniqueId": "CSGO",
				"Attribution": {
					"Image": "img/gs/cs/leetify.webp",
					"Dimensions": "270x115",
					"Link": "https://leetify.com"
				}
			},
			"Apex Legends": {
				"UniqueId": "AL",
				"Attribution": {
					"Text": "Apex Legends Status",
					"Link": "https://apexlegendsstatus.com"
				}
			}
		}
	}`)
	var cfg gameStatsFile
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	cs := cfg.StatsDefinitions["Counter-Strike 2"]
	if cs.Attribution == nil || cs.Attribution.Image != "img/gs/cs/leetify.webp" || cs.Attribution.Link != "https://leetify.com" || cs.Attribution.Dimensions != "270x115" {
		t.Fatalf("cs attribution: %+v", cs.Attribution)
	}
	dto := gameAttributionToDTO(cs.Attribution)
	if dto.Image != "img/gs/cs/leetify.webp" || dto.Link != "https://leetify.com" || dto.Dimensions != "270x115" || dto.Header != defaultGameStatAttributionHeader {
		t.Fatalf("cs dto: %+v", dto)
	}
	dtoEmptyHeader := gameAttributionToDTO(&gameAttribution{Link: "https://example.com"})
	if dtoEmptyHeader.Header != defaultGameStatAttributionHeader {
		t.Fatalf("empty header dto: %+v", dtoEmptyHeader)
	}
	al := cfg.StatsDefinitions["Apex Legends"]
	if al.Attribution == nil || al.Attribution.Text != "Apex Legends Status" {
		t.Fatalf("al attribution: %+v", al.Attribution)
	}
}
