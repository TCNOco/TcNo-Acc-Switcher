package basic

import (
	"strings"
	"sync"
)

// StandaloneStatsProvider yields a stats payload for an account that has no
// configured game stats row, in the same shape the game's variant 0 parses.
//
// It exists so a first-party source can put metrics on the tile without the user
// having set the game up for that account - the CS2 ranks the Steam Guard sweep
// already reads are the case it was built for. Returning false means "nothing
// for this account", which leaves the tile as it was.
type StandaloneStatsProvider func(accountID string) ([]byte, bool)

var (
	standaloneStatsMu        sync.RWMutex
	standaloneStatsProviders = map[string]StandaloneStatsProvider{}
)

func standaloneStatsKey(platformKey, game string) string {
	return strings.TrimSpace(platformKey) + "\x00" + strings.TrimSpace(game)
}

// RegisterStandaloneStats registers a provider for one platform and game. Call
// once at startup.
func RegisterStandaloneStats(platformKey, game string, provider StandaloneStatsProvider) {
	if provider == nil {
		return
	}
	standaloneStatsMu.Lock()
	defer standaloneStatsMu.Unlock()
	standaloneStatsProviders[standaloneStatsKey(platformKey, game)] = provider
}

func standaloneStatsProviderFor(platformKey, game string) StandaloneStatsProvider {
	standaloneStatsMu.RLock()
	defer standaloneStatsMu.RUnlock()
	return standaloneStatsProviders[standaloneStatsKey(platformKey, game)]
}

// standaloneStatsMarkup renders one game's metrics for an account with no cached
// row.
//
// It runs the payload through collectStatsFromHTML and collectIndicatorMarkup -
// the very functions the configured path uses - so the markup is identical by
// construction rather than by a second implementation kept in step by hand.
func standaloneStatsMarkup(platformKey, game, accountID string, def gameDefinition) map[string]StatValueAndIconDTO {
	provider := standaloneStatsProviderFor(platformKey, game)
	if provider == nil {
		return nil
	}
	payload, ok := provider(accountID)
	if !ok || len(payload) == 0 {
		return nil
	}
	// Variant 0 is the first-party source the provider speaks for; the fallback
	// variants describe third-party APIs it knows nothing about.
	def = def.variantAt(0)
	collected, err := collectStatsFromHTML(platformKey, accountID, def, nil, payload)
	if err != nil || len(collected) == 0 {
		return nil
	}
	out := make(map[string]StatValueAndIconDTO, len(collected))
	for key, value := range collected {
		ci, known := def.Collect[key]
		if !known {
			continue
		}
		out[key] = StatValueAndIconDTO{
			StatValue:       value,
			IndicatorMarkup: collectIndicatorMarkup(ci, def.Indicator),
			Tooltip:         strings.TrimSpace(ci.Tooltip),
		}
	}
	return out
}
