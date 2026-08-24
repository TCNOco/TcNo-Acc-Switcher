//go:build windows

// The fake backend looks portable, but the fixtures are Windows window handles
// and C:\ paths, and the only real backend is platform_windows.go.
package qrcapture

import (
	"errors"
	"testing"
)

type fakeBackend struct {
	registry      []string
	processes     []ProcessInfo
	roots         map[string]string
	executables   map[string]string
	windows       []WindowInfo
	monitors      []Rect
	captured      []Rect
	platformError error
}

func (f *fakeBackend) RegistrySteamPaths() ([]string, error) { return f.registry, f.platformError }
func (f *fakeBackend) RunningProcesses() ([]ProcessInfo, error) {
	return f.processes, f.platformError
}
func (f *fakeBackend) CanonicalSteamRoot(hint string) (string, error) {
	if root := f.roots[hint]; root != "" {
		return root, nil
	}
	return "", errors.New("invalid root")
}
func (f *fakeBackend) CanonicalExecutable(path string) (string, error) {
	if executable := f.executables[path]; executable != "" {
		return executable, nil
	}
	return "", errors.New("invalid executable")
}
func (f *fakeBackend) TopLevelWindows() ([]WindowInfo, error) { return f.windows, f.platformError }
func (f *fakeBackend) MonitorBounds() ([]Rect, error)         { return f.monitors, f.platformError }
func (f *fakeBackend) CaptureRegion(region Rect) (Frame, error) {
	f.captured = append(f.captured, region)
	return Frame{Region: region, Width: int(region.Width()), Height: int(region.Height())}, nil
}

func fixtureBackend() *fakeBackend {
	const root = `C:\Steam`
	return &fakeBackend{
		registry: []string{root},
		roots: map[string]string{
			root:                                  root,
			`C:\Steam\steam.exe`:                  root,
			`C:\Steam\bin\cef\steamwebhelper.exe`: root,
		},
		processes: []ProcessInfo{
			{PID: 10, ExecutablePath: `C:\Steam\steam.exe`},
			{PID: 11, ExecutablePath: `C:\Steam\bin\cef\steamwebhelper.exe`},
			{PID: 12, ExecutablePath: `C:\SteamEvil\steam.exe`},
			{PID: 99, ExecutablePath: `C:\Steam\steam.exe`},
		},
		executables: map[string]string{
			`C:\Steam\steam.exe`:                  `C:\Steam\steam.exe`,
			`C:\Steam\bin\cef\steamwebhelper.exe`: `C:\Steam\bin\cef\steamwebhelper.exe`,
			`C:\SteamEvil\steam.exe`:              `C:\SteamEvil\steam.exe`,
		},
		monitors: []Rect{{Left: 0, Top: 0, Right: 800, Bottom: 600}, {Left: 800, Top: 0, Right: 1600, Bottom: 600}},
		windows: []WindowInfo{
			{Handle: 1, PID: 10, Visible: true, Bounds: Rect{Left: -100, Top: 0, Right: 1100, Bottom: 500}, Title: "Steam"},
			{Handle: 2, Owner: 1, PID: 11, Visible: true, Bounds: Rect{Left: 100, Top: 100, Right: 500, Bottom: 500}, Title: "Sign in"},
			{Handle: 3, PID: 10, Visible: true, Cloaked: true, Bounds: Rect{Left: 0, Top: 0, Right: 100, Bottom: 100}},
			{Handle: 4, PID: 10, Visible: true, Bounds: Rect{}},
			{Handle: 5, PID: 10, Visible: true, Bounds: Rect{Left: -500, Top: -500, Right: -100, Bottom: -100}},
			{Handle: 6, PID: 10, Visible: true, Bounds: Rect{Left: 0, Top: 0, Right: 100, Bottom: 100}, Title: "TcNo Account Switcher"},
			{Handle: 7, PID: 12, Visible: true, Bounds: Rect{Left: 0, Top: 0, Right: 100, Bottom: 100}},
			{Handle: 8, PID: 99, Visible: true, Bounds: Rect{Left: 0, Top: 0, Right: 100, Bottom: 100}},
		},
	}
}

func TestDiscoverVerifiesIdentityAndIncludesOwnedDialogs(t *testing.T) {
	backend := fixtureBackend()
	scanner := NewWithBackend(backend, 99)
	discovery, err := scanner.Discover(`C:\Steam`)
	if err != nil {
		t.Fatal(err)
	}
	if discovery.State != DiscoveryReady || len(discovery.Roots) != 1 {
		t.Fatalf("unexpected discovery: %#v", discovery)
	}
	if len(discovery.Windows) != 2 {
		t.Fatalf("windows = %d, want 2", len(discovery.Windows))
	}
	if discovery.Windows[0].Handle != 2 || discovery.Windows[0].Owner != 1 {
		t.Fatalf("owned dialog was not retained: %#v", discovery.Windows[0])
	}
	main := discovery.Windows[1]
	if len(main.Regions) != 2 || main.Regions[0] != (Rect{Left: 0, Top: 0, Right: 800, Bottom: 500}) {
		t.Fatalf("monitor clipping failed: %#v", main.Regions)
	}
}

func TestCaptureRevalidatesIdentityAndWindow(t *testing.T) {
	backend := fixtureBackend()
	scanner := NewWithBackend(backend, 99)
	discovery, err := scanner.Discover(`C:\Steam`)
	if err != nil {
		t.Fatal(err)
	}
	candidate := discovery.Windows[1]
	capture, err := scanner.CaptureWindow(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if capture.State != CaptureReady || len(capture.Frames) != 2 || len(backend.captured) != 2 {
		t.Fatalf("unexpected capture: %#v", capture)
	}

	backend.processes[0].ExecutablePath = `C:\SteamEvil\steam.exe`
	capture, err = scanner.CaptureWindow(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if capture.State != CaptureUnavailable || len(backend.captured) != 2 {
		t.Fatalf("stale identity was captured: %#v", capture)
	}
}

func TestDiscoveryTypedStates(t *testing.T) {
	backend := fixtureBackend()
	backend.roots = map[string]string{}
	discovery, err := NewWithBackend(backend, 99).Discover("")
	if err != nil || discovery.State != DiscoverySteamNotFound {
		t.Fatalf("missing root state: %#v, %v", discovery, err)
	}

	unsupported := &fakeBackend{platformError: &UnsupportedError{GOOS: "test"}}
	discovery, err = NewWithBackend(unsupported, 99).Discover("")
	if !errors.Is(err, ErrUnsupported) || discovery.State != DiscoveryUnsupported {
		t.Fatalf("unsupported state: %#v, %v", discovery, err)
	}
}

func TestPathWithinRootRejectsSiblingAndParent(t *testing.T) {
	for _, path := range []string{`C:\SteamEvil\steam.exe`, `C:\steam.exe`, `D:\Steam\steam.exe`} {
		if pathWithinRoot(`C:\Steam`, path) {
			t.Fatalf("escaped path accepted: %s", path)
		}
	}
	if !pathWithinRoot(`C:\Steam`, `C:\Steam\bin\steamwebhelper.exe`) {
		t.Fatal("valid child rejected")
	}
}
