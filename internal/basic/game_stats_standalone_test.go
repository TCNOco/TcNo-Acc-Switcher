package basic

import "testing"

func standaloneTestDef() gameDefinition {
	return gameDefinition{
		Collect: map[string]collectInstruction{
			"Premiere": {Source: "json", Path: "premier", DisplayAs: "<div class='cs-premier'>%x_fmt%</div>", DisplayFormat: "commaNumber"},
			"CompRank": {Source: "json", Path: "compRank", DisplayAs: `<img src="img/gs/cs/comp%x%.webp"/>`},
		},
	}
}

// A provider-served metric lands in the same tile row as one from a configured
// refresh, so it must render through the same collectStatsFromHTML and DisplayAs
// markup, not a second implementation.
func TestStandaloneStatsMarkupRendersTheDefinitionsDisplayMarkup(t *testing.T) {
	RegisterStandaloneStats("StandalonePlatform", "StandaloneGame", func(accountID string) ([]byte, bool) {
		if accountID != "acct-1" {
			return nil, false
		}
		return []byte(`{"premier":22887,"compRank":12}`), true
	})

	got := standaloneStatsMarkup("StandalonePlatform", "StandaloneGame", "acct-1", standaloneTestDef())
	if want := "<div class='cs-premier'>22,887</div>"; got["Premiere"].StatValue != want {
		t.Fatalf("Premiere = %q, want %q", got["Premiere"].StatValue, want)
	}
	if want := `<img src="img/gs/cs/comp12.webp"/>`; got["CompRank"].StatValue != want {
		t.Fatalf("CompRank = %q, want %q", got["CompRank"].StatValue, want)
	}
}

func TestStandaloneStatsMarkupYieldsNothingWhenTheProviderDeclines(t *testing.T) {
	RegisterStandaloneStats("DecliningPlatform", "StandaloneGame", func(string) ([]byte, bool) {
		return nil, false
	})
	if got := standaloneStatsMarkup("DecliningPlatform", "StandaloneGame", "acct-1", standaloneTestDef()); got != nil {
		t.Fatalf("markup = %v, want none", got)
	}
}

// An unregistered platform or game must cost nothing and draw nothing, since
// every game on every account list reaches this.
func TestStandaloneStatsMarkupYieldsNothingWithoutAProvider(t *testing.T) {
	if got := standaloneStatsMarkup("UnregisteredPlatform", "StandaloneGame", "acct-1", standaloneTestDef()); got != nil {
		t.Fatalf("markup = %v, want none", got)
	}
}
