package steam

// SteamGuardAccountState is the non-secret account-list projection of the
// Steam Guard vault registration state.
type SteamGuardAccountState struct {
	HasSteamGuard bool
	Pending       bool
	// LoginOnly is a vault record holding a Steam session but no authenticator.
	// It is deliberately not HasSteamGuard: there is no code to show and no
	// confirmations to approve, so the lock icon would be a lie. It still counts
	// as "in the vault" for reaching the Steam Guard window.
	LoginOnly bool
}

// InVault reports whether the account has any Steam Guard vault record.
func (s SteamGuardAccountState) InVault() bool {
	return s.HasSteamGuard || s.Pending || s.LoginOnly
}
