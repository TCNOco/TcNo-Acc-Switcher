package gcpd

import (
	"os"
	"testing"
	"time"
)

// TestParseAgainstADumpedPage runs the parser over a real captured response.
// Opt-in: set TCNO_GCPD_FIXTURE to a dumped .html file.
func TestParseAgainstADumpedPage(t *testing.T) {
	path := os.Getenv("TCNO_GCPD_FIXTURE")
	if path == "" {
		t.Skip("set TCNO_GCPD_FIXTURE=<dumped .html> to check the parser against a real page")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := Parse(body, time.Now())
	t.Logf("outcome=%s hasCooldown=%v permanent=%v expiresAt=%v",
		got.Outcome, got.HasCooldown, got.Permanent, got.ExpiresAt.Format(time.RFC3339))
	t.Logf("premier=%+v wingman=%+v competitive=%+v",
		got.Ranks.Premier, got.Ranks.Wingman, got.Ranks.Competitive)
	// PremierPlayed decides Prime on its own, and is the one field that can be
	// true while every rank is absent - the case that read as Non-Prime.
	t.Logf("premierPlayed=%v hasGameData=%v", got.Ranks.PremierPlayed, got.HasGameData)
}
