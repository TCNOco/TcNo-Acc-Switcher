//go:build !windows

package steam

// steamNativeQuit has no spawned equivalent off Windows: SIGTERM is how a Unix
// process is asked to shut itself down, and KillByNameWithOpts already sends it
// as the first escalation step.
//
// "steam.sh -shutdown" is no alternative: it boots a second launcher instead of
// signalling the running one, so it only spends the native-quit deadline before
// the signal that was always going to do it.
func steamNativeQuit(string) func() error { return nil }
