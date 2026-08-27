package primestatus

import (
	"os"
	"testing"
)

// Opt-in: set TCNO_STORE_FIXTURE to a dumped store page.
//
// The synthetic fixtures use hand-written markup, so only a page Steam actually
// served catches a marker that never existed on one.
func TestParseAgainstADumpedStorePage(t *testing.T) {
	path := os.Getenv("TCNO_STORE_FIXTURE")
	if path == "" {
		t.Skip("set TCNO_STORE_FIXTURE=<dumped .html> to check the parser against a real page")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := Parse(body)
	t.Logf("bytes=%d outcome=%s ownsPrimePackage=%v", len(body), got.Outcome, got.OwnsPrimePackage)
	if got.Outcome == OutcomeUnrecognised {
		t.Fatal("parser did not recognise a real store page")
	}
}
