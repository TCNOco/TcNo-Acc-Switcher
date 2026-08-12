package screenprivacy

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func reset(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		enabled.Store(false)
		mu.Lock()
		followed = map[uint]application.Window{}
		mu.Unlock()
	})
	enabled.Store(false)
	mu.Lock()
	followed = map[uint]application.Window{}
	mu.Unlock()
}

func TestApplyProtectsNewWindowsWhileEnabled(t *testing.T) {
	reset(t)

	var off application.WebviewWindowOptions
	Apply(&off)
	if off.ContentProtectionEnabled {
		t.Fatal("a window was protected with the preference off")
	}

	enabled.Store(true)
	var on application.WebviewWindowOptions
	Apply(&on)
	if !on.ContentProtectionEnabled {
		t.Fatal("a window was left capturable with the preference on")
	}
}

// Steam Guard's windows set the flag themselves and are never registered here.
// If Apply cleared it, routing their options through this package by mistake
// would silently strip the protection that keeps codes out of screen shares.
func TestApplyNeverClearsProtectionAWindowAskedFor(t *testing.T) {
	reset(t)

	options := application.WebviewWindowOptions{ContentProtectionEnabled: true}
	Apply(&options)
	if !options.ContentProtectionEnabled {
		t.Fatal("Apply stripped protection the window had asked for")
	}
}

func TestSetEnabledWithoutARunningAppIsHarmless(t *testing.T) {
	reset(t)

	// Called at startup before any window exists, and by tests and headless runs
	// where application.Get() is nil.
	SetEnabled(true)
	if !Enabled() {
		t.Fatal("the preference was not recorded")
	}
	SetEnabled(false)
	if Enabled() {
		t.Fatal("the preference was not cleared")
	}
}
