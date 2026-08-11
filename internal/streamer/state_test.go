package streamer

import "testing"

// withStubbedWatcher swaps the Win32 detector out and resets the package state, so
// each test starts from a known point and never installs a real hook.
func withStubbedWatcher(t *testing.T) *[]bool {
	t.Helper()
	prev := watchFn
	var calls []bool
	watchFn = func(on bool) { calls = append(calls, on) }
	t.Cleanup(func() {
		watchFn = prev
		state.mu.Lock()
		state.manual, state.autoEnabled, state.autoActive, state.detectedExe, state.hook = false, false, false, "", nil
		state.mu.Unlock()
	})
	state.mu.Lock()
	state.manual, state.autoEnabled, state.autoActive, state.detectedExe, state.hook = false, false, false, "", nil
	state.mu.Unlock()
	return &calls
}

func TestEffectiveFollowsOverrideAndDetection(t *testing.T) {
	withStubbedWatcher(t)

	SetAutoEnabled(true)
	if Current().Effective {
		t.Fatal("auto enabled with nothing detected should not censor")
	}

	setDetected(true, "obs64.exe")
	if got := Current(); !got.Effective || got.DetectedExe != "obs64.exe" {
		t.Fatalf("detected broadcaster should censor: %+v", got)
	}

	setDetected(false, "")
	if Current().Effective {
		t.Fatal("broadcaster exit should stop censoring")
	}

	// The override has to win on its own, with detection idle.
	SetManual(true)
	SetAutoEnabled(false)
	if !Current().Effective {
		t.Fatal("manual override should censor regardless of detection")
	}
}

func TestDisablingAutoClearsStaleDetection(t *testing.T) {
	withStubbedWatcher(t)

	SetAutoEnabled(true)
	setDetected(true, "obs64.exe")
	SetAutoEnabled(false)

	// Re-enabling must not resurrect a detection from before the watcher stopped:
	// the process it saw may have exited while nothing was listening.
	SetAutoEnabled(true)
	if got := Current(); got.AutoActive || got.Effective || got.DetectedExe != "" {
		t.Fatalf("stale detection survived a disable/enable cycle: %+v", got)
	}
}

func TestChangeHookFiresOnlyOnRealChanges(t *testing.T) {
	withStubbedWatcher(t)

	var fired []State
	SetChangeHook(func(s State) { fired = append(fired, s) })

	SetManual(true)
	SetManual(true)
	SetManual(false)

	if len(fired) != 2 {
		t.Fatalf("expected 2 notifications for on/off, got %d", len(fired))
	}
	if !fired[0].Effective || fired[1].Effective {
		t.Fatalf("notifications carry the wrong effective state: %+v", fired)
	}
}

func TestWatcherRunsOnlyWhileAutoIsEnabled(t *testing.T) {
	calls := withStubbedWatcher(t)

	SetAutoEnabled(true)
	SetManual(true)
	SetAutoEnabled(false)

	// The manual override must not keep the detector alive; it censors on its own.
	if last := (*calls)[len(*calls)-1]; last {
		t.Fatal("detector still asked to run after auto was turned off")
	}
	for _, on := range (*calls)[:1] {
		if !on {
			t.Fatal("enabling auto should start the detector")
		}
	}
}

func TestVirtualAdaptersAreNotSaltMaterial(t *testing.T) {
	cases := []struct {
		name    string
		mac     string
		virtual bool
	}{
		{"Ethernet", "d8:5e:d3:11:22:33", false},
		{"Wi-Fi", "9c:b6:d0:aa:bb:cc", false},
		{"VMware Network Adapter VMnet1", "00:50:56:c0:00:01", true},
		{"Ethernet", "08:00:27:1a:2b:3c", true},   // VirtualBox OUI on an innocent name
		{"vEthernet (Default Switch)", "00:15:5d:01:02:03", true},
		{"Wi-Fi", "9e:b6:d0:aa:bb:cc", true},      // randomised: locally administered
		{"Local Area Connection", "02:11:22:33:44:55", true},
	}
	for _, tc := range cases {
		if got := isVirtualAdapter(tc.name, tc.mac); got != tc.virtual {
			t.Errorf("isVirtualAdapter(%q, %q) = %v, want %v", tc.name, tc.mac, got, tc.virtual)
		}
	}
}
