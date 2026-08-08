package steamguard

import (
	"TcNo-Acc-Switcher/internal/steamguard/gcpd"
	"TcNo-Acc-Switcher/internal/steamguard/primestatus"
)

// Prime states written to the store and sent to the account list. The empty
// string is "we do not know", and is what every account starts as.
const (
	PrimeStateUnknown  = ""
	PrimeStatePrime    = "prime"
	PrimeStateNonPrime = "nonprime"
)

// decidePrimeState combines the two things that bear on Prime.
//
// Only the positive side is sound. Owning the Prime Status Upgrade package
// proves Prime, and so does any Premier history, because Premier is Prime-gated.
// There is no equivalent proof of absence: accounts that reached rank 21 before
// 2019 were granted Prime without ever holding the package, and one that stopped
// playing before Premier existed (2023) shows neither signal. A verified account
// with 84 Premier wins and no package is exactly that case.
//
// Non-Prime is therefore a best-effort guess, requested deliberately. It is
// withheld wherever it would clearly be wrong: an unreadable page, a lapsed
// session, or an account Steam holds no CS2 records for at all - the last
// because "never played" says nothing about what was bought.
func decidePrimeState(store primestatus.Result, ranks gcpd.Ranks, hasGameData bool) string {
	if ranks.PremierPlayed || ranks.Premier.Found {
		return PrimeStatePrime
	}
	if store.Outcome != primestatus.OutcomeParsed {
		return PrimeStateUnknown
	}
	if store.OwnsPrimePackage {
		return PrimeStatePrime
	}
	if !hasGameData {
		return PrimeStateUnknown
	}
	return PrimeStateNonPrime
}
