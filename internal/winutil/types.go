package winutil

import (
	"errors"
	"log/slog"
)

func slogWin() *slog.Logger {
	return slog.Default().With("component", "winutil")
}

// ErrUnsupported is returned by stubs on non-Windows builds.
var ErrUnsupported = errors.New("winutil: unsupported on this platform")

var errRegParse = errors.New("winutil: invalid registry path")

// Win32 registry value types (REG_*), for JSON hints without importing golang.org/x/sys/windows/registry.
const (
	RegValueTypeNone     uint32 = 0
	RegValueTypeSZ       uint32 = 1
	RegValueTypeExpandSZ uint32 = 2
	RegValueTypeBinary   uint32 = 3
	RegValueTypeDWORD    uint32 = 4
	RegValueTypeQWORD    uint32 = 11
	RegValueTypeMultiSZ  uint32 = 7
)

// ClosingMethod matches platform settings: Combined, Close, TaskKill, Electron.
type ClosingMethod string

const (
	ClosingCombined ClosingMethod = "Combined"
	ClosingClose    ClosingMethod = "Close"
	ClosingTaskKill ClosingMethod = "TaskKill"
	// ClosingElectron: optional prepare hook (launch platform), Alt+F4 per browser root, then taskkill; no WM_CLOSE.
	ClosingElectron ClosingMethod = "Electron"
)

// KillOpts tunes how KillByName ends a platform.
type KillOpts struct {
	// NativeQuit, when non-nil, issues the platform's own quit command once before any
	// window messages are sent, and replaces the WM_CLOSE broadcast as the graceful signal.
	//
	// Steam needs this: it treats WM_CLOSE on its main window as "minimise to tray", so the
	// WM_CLOSE path can never make it exit and the graceful window burns its full deadline
	// on every single switch. "steam.exe -shutdown" drains the whole tree in ~1.6-2.5s.
	NativeQuit func() error
	// BeforeElectronSynth runs before Electron Alt+F4 (e.g. launch platform + wait for foreground).
	BeforeElectronSynth func() error
}

// StartOpts controls process creation.
type StartOpts struct {
	Admin         bool
	HideWindow    bool
	WorkingDir    string
	AsDesktopUser bool // drop elevation when switcher is admin but target should not inherit admin
}
