//go:build !windows

package steam

// steamNativeQuit has no spawned equivalent off Windows: SIGTERM is how a Unix
// process is asked to shut itself down, and KillByNameWithOpts already sends it
// as the first escalation step.
//
// Running the launcher with a shutdown argument is a Windows idiom and does not
// carry over. Measured on a signed-in Linux Steam, "steam.sh -shutdown" boots a
// whole second launcher instead of signalling the running one and never makes it
// exit - still alive after 40s - so offering it here only spends the native-quit
// deadline before the signal that was always going to do it. SIGTERM drains
// Steam and its nine steamwebhelper children in ~2.3s.
func steamNativeQuit(string) func() error { return nil }
