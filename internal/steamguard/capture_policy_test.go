package steamguard

import "testing"

// The protection is what keeps codes and trades out of screen shares, so it has
// to be on unless this run was explicitly told otherwise.
func TestSteamGuardWindowsAreHiddenFromCaptureByDefault(t *testing.T) {
	if !contentProtectionEnabled() {
		t.Fatal("capture protection is off without the flag")
	}
	if !confirmationsWindowOptions("user").ContentProtectionEnabled {
		t.Fatal("the confirmations window is capturable without the flag")
	}
}

// Not parallel and restored afterwards: the switch is process-wide by design, so
// it cannot be turned off by anything the app does after startup.
func TestResolveCapturePolicyOptsOneRunOut(t *testing.T) {
	t.Cleanup(func() { steamGuardCaptureAllowed.Store(false) })

	ResolveCapturePolicy(true)
	if contentProtectionEnabled() {
		t.Fatal("the flag did not lift the protection")
	}
	if confirmationsWindowOptions("user").ContentProtectionEnabled {
		t.Fatal("the confirmations window is still protected with the flag set")
	}
}

// wails3 dev cannot forward arguments to the app, so the variable is the only way
// in while developing.
func TestResolveCapturePolicyHonoursTheEnvironment(t *testing.T) {
	t.Cleanup(func() { steamGuardCaptureAllowed.Store(false) })

	t.Setenv(captureEnv, "1")
	ResolveCapturePolicy(false)
	if contentProtectionEnabled() {
		t.Fatal("the environment variable did not lift the protection")
	}
}

// Neither asked for it, so the protection stays on.
func TestResolveCapturePolicyLeavesProtectionAloneOtherwise(t *testing.T) {
	t.Cleanup(func() { steamGuardCaptureAllowed.Store(false) })

	t.Setenv(captureEnv, "0")
	ResolveCapturePolicy(false)
	if !contentProtectionEnabled() {
		t.Fatal("the protection was lifted without being asked")
	}
}
