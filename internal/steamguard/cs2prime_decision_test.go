package steamguard

import (
	"testing"

	"TcNo-Acc-Switcher/internal/steamguard/gcpd"
	"TcNo-Acc-Switcher/internal/steamguard/primestatus"
)

func parsedStore(owns bool) primestatus.Result {
	return primestatus.Result{Outcome: primestatus.OutcomeParsed, OwnsPrimePackage: owns}
}

func TestDecidePrimeState(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		store       primestatus.Result
		ranks       gcpd.Ranks
		hasGameData bool
		want        string
	}{{
		name:  "owning the package proves Prime",
		store: parsedStore(true),
		want:  PrimeStatePrime,
	}, {
		// Verified Prime, 84 Premier wins, package not owned because it was
		// granted pre-2019.
		name:        "Premier history outweighs an unowned package",
		store:       parsedStore(false),
		ranks:       gcpd.Ranks{Premier: gcpd.Rank{Found: true, Value: 22887, Wins: 84}},
		hasGameData: true,
		want:        PrimeStatePrime,
	}, {
		// Placements: matches played, rating not yet earned, so the rank itself
		// is not Found. Premier is still Prime-gated.
		name:        "Premier in placements still proves Prime",
		store:       parsedStore(false),
		ranks:       gcpd.Ranks{PremierPlayed: true},
		hasGameData: true,
		want:        PrimeStatePrime,
	}, {
		name:        "played, no Premier, no package is the best-effort guess",
		store:       parsedStore(false),
		ranks:       gcpd.Ranks{Competitive: gcpd.Rank{Found: true, Value: 5}},
		hasGameData: true,
		want:        PrimeStateNonPrime,
	}, {
		// Nothing was bought or played, so there is nothing to guess from.
		name:  "never played stays unknown",
		store: parsedStore(false),
		want:  PrimeStateUnknown,
	}, {
		name:        "an unreadable store page stays unknown",
		store:       primestatus.Result{Outcome: primestatus.OutcomeUnrecognised},
		hasGameData: true,
		want:        PrimeStateUnknown,
	}, {
		// A lapsed session renders every section unowned; without this it would
		// relabel every account as Non-Prime at once.
		name:        "a signed-out store page stays unknown",
		store:       primestatus.Result{Outcome: primestatus.OutcomeNotSignedIn},
		hasGameData: true,
		want:        PrimeStateUnknown,
	}, {
		// Premier is read from GCPD, which the sweep always fetches, so it stands
		// even when the extra store request failed.
		name:        "Premier still proves Prime when the store page failed",
		store:       primestatus.Result{Outcome: primestatus.OutcomeUnrecognised},
		ranks:       gcpd.Ranks{PremierPlayed: true},
		hasGameData: true,
		want:        PrimeStatePrime,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := decidePrimeState(tc.store, tc.ranks, tc.hasGameData); got != tc.want {
				t.Fatalf("decidePrimeState = %q, want %q", got, tc.want)
			}
		})
	}
}
