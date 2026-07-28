package steamguard

import (
	"os"
	"strings"
	"sync/atomic"
)

// Steam Guard windows are excluded from screen capture: a code, a trade and the
// items in it should not land in a screen share, a recording or a remote-desktop
// session by accident.
//
// That exclusion also blocks deliberate capture, which is a problem for
// screenshots of the feature itself, so it can be lifted for a run with either
// --allow-steamguard-capture or TCNO_ALLOW_STEAMGUARD_CAPTURE. It is a
// process-wide switch set once at startup rather than a setting, so it cannot be
// turned off by anything the app does later, and it leaves a line in the log
// saying the protection is off.
var steamGuardCaptureAllowed atomic.Bool

// captureEnv lifts the protection the same way the flag does. It exists because
// `wails3 dev` runs the app through a task and cannot forward arguments to it,
// so a flag alone is unusable while developing.
const captureEnv = "TCNO_ALLOW_STEAMGUARD_CAPTURE"

// ResolveCapturePolicy decides once, at startup, whether Steam Guard windows are
// hidden from screen capture. Call it before any of them exists.
func ResolveCapturePolicy(flagSet bool) {
	source := ""
	switch {
	case flagSet:
		source = "--allow-steamguard-capture"
	case envSet(captureEnv):
		source = captureEnv
	default:
		return
	}
	steamGuardCaptureAllowed.Store(true)
	serviceLogger().Warn("Steam Guard screen-capture protection is disabled for this run",
		"source", source)
}

func envSet(name string) bool {
	value := strings.TrimSpace(os.Getenv(name))
	return value != "" && value != "0" && !strings.EqualFold(value, "false")
}

// contentProtectionEnabled reports whether Steam Guard windows should be hidden
// from screen capture.
func contentProtectionEnabled() bool {
	return !steamGuardCaptureAllowed.Load()
}
