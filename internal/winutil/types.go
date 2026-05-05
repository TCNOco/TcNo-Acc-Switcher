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

// StartingMethod matches platform settings: Default, Direct (both launch the exe; shim optional).
type StartingMethod string

const (
	StartingDefault StartingMethod = "Default"
	StartingDirect  StartingMethod = "Direct"
)

// StartOpts controls process creation.
type StartOpts struct {
	Admin         bool
	Method        StartingMethod
	HideWindow    bool
	WorkingDir    string
	AsDesktopUser bool // drop elevation when switcher is admin but target should not inherit admin
}
