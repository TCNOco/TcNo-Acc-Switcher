package winutil

import "errors"

// ErrUnsupported is returned by stubs on non-Windows builds.
var ErrUnsupported = errors.New("winutil: unsupported on this platform")

var errRegParse = errors.New("winutil: invalid registry path")

// ClosingMethod matches platform settings: Combined, Close, TaskKill.
type ClosingMethod string

const (
	ClosingCombined ClosingMethod = "Combined"
	ClosingClose    ClosingMethod = "Close"
	ClosingTaskKill ClosingMethod = "TaskKill"
)

// StartingMethod matches platform settings: Default, Direct (both launch the exe; shim optional).
type StartingMethod string

const (
	StartingDefault StartingMethod = "Default"
	StartingDirect  StartingMethod = "Direct"
)

// StartOpts controls process creation.
type StartOpts struct {
	Admin          bool
	Method         StartingMethod
	HideWindow     bool
	WorkingDir     string
	AsDesktopUser  bool // drop elevation when switcher is admin but target should not inherit admin
}
