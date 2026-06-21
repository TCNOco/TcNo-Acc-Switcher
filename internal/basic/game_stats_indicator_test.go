package basic

import (
	"encoding/json"
	"testing"
)

func TestCollectIndicatorMarkup(t *testing.T) {
	t.Parallel()
	empty := ""
	apex := "APEX"
	if got := collectIndicatorMarkup(collectInstruction{}, "APEX"); got != "<sup>APEX</sup>" {
		t.Fatalf("inherit game: %q", got)
	}
	if got := collectIndicatorMarkup(collectInstruction{Indicator: &empty}, "APEX"); got != "" {
		t.Fatalf("suppress: %q", got)
	}
	if got := collectIndicatorMarkup(collectInstruction{Indicator: &apex}, ""); got != "<sup>APEX</sup>" {
		t.Fatalf("override: %q", got)
	}
	if got := collectIndicatorMarkup(collectInstruction{Icon: "<b>x</b>"}, "APEX"); got != "<b>x</b>" {
		t.Fatalf("icon wins: %q", got)
	}
}

func TestCollectInstructionIndicatorJSON(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"Indicator":""}`)
	var ci collectInstruction
	if err := json.Unmarshal(raw, &ci); err != nil {
		t.Fatal(err)
	}
	if ci.Indicator == nil || *ci.Indicator != "" {
		t.Fatalf("expected pointer to empty string, got %+v", ci.Indicator)
	}
	var ci2 collectInstruction
	if err := json.Unmarshal([]byte(`{}`), &ci2); err != nil {
		t.Fatal(err)
	}
	if ci2.Indicator != nil {
		t.Fatalf("expected nil indicator, got %+v", ci2.Indicator)
	}
}
