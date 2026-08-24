package platform

import (
	"runtime"
	"testing"

	"TcNo-Acc-Switcher/internal/winutil"
)

// A capability table is only useful if it agrees with the code it describes.
// These check the two that are easy to get wrong because they differ between
// Linux and macOS, by asking the implementation rather than trusting the table.
func TestProcessControlMatchesWinutil(t *testing.T) {
	// Start on a path that cannot exist tells the two apart without launching
	// anything: the stub refuses with ErrUnsupported before looking at the file,
	// while a real implementation gets far enough to fail on the exe itself.
	err := winutil.Start("", nil, winutil.StartOpts{})
	stubbed := err != nil && errorIsUnsupported(err)
	if got := Capabilities().ProcessControl; got == stubbed {
		t.Errorf("ProcessControl = %v on %s, but winutil.Start stubbed = %v", got, runtime.GOOS, stubbed)
	}
}

func TestCapabilitiesAgreeWithGOOS(t *testing.T) {
	c := Capabilities()
	if runtime.GOOS == "windows" {
		if !c.Shortcuts || !c.Registry || !c.Elevation {
			t.Error("Windows should support shortcuts, registry and elevation")
		}
		return
	}
	// Everything Win32-only must be off, on both Linux and macOS.
	for name, on := range map[string]bool{
		"Shortcuts":          c.Shortcuts,
		"Elevation":          c.Elevation,
		"Registry":           c.Registry,
		"ClosingMethods":     c.ClosingMethods,
		"ProtocolHandler":    c.ProtocolHandler,
		"BroadcastDetection": c.BroadcastDetection,
		"ControllerInput":    c.ControllerInput,
		"QRCapture":          c.QRCapture,
		"SecureClipboard":    c.SecureClipboard,
		"SteamBrowser":       c.SteamBrowser,
		"ServerPicker":       c.ServerPicker,
	} {
		if on {
			t.Errorf("%s claims support on %s, but its implementation is Windows-only", name, runtime.GOOS)
		}
	}
	// Autostart is the one thing Wails implements on all three.
	if !c.Autostart {
		t.Errorf("Autostart should be supported on %s", runtime.GOOS)
	}
}

// ClosingMethods must agree with the normaliser, or the UI offers choices the
// backend collapses.
func TestClosingMethodsCapabilityMatchesNormaliser(t *testing.T) {
	honoured := NormalizeClosingMethod("TaskKill") == "TaskKill"
	if got := Capabilities().ClosingMethods; got != honoured {
		t.Errorf("ClosingMethods = %v but NormalizeClosingMethod(TaskKill) honoured = %v", got, honoured)
	}
}

func errorIsUnsupported(err error) bool {
	for e := err; e != nil; {
		if e == winutil.ErrUnsupported {
			return true
		}
		u, ok := e.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}
