package steam

// SteamGuardAccountState is the non-secret account-list projection of the
// Steam Guard vault registration state.
type SteamGuardAccountState struct {
	HasSteamGuard bool
	Pending       bool
}
