package platform

import "runtime"

// OSCapabilities tells the frontend what this build can actually do, so a
// control is hidden because the capability behind it is missing rather than
// because of what OS the webview thinks it is on.
//
// Values live in the per-OS files beside this one. A capability is true only
// when the action behind it actually happens - a stub that returns nil without
// doing anything is false, not true.
type OSCapabilities struct {
	// Shortcuts covers desktop and game shortcuts. Off Windows,
	// winutil.WriteShortcutLnk returns ErrUnsupported: .lnk is the only format
	// implemented.
	Shortcuts bool `json:"shortcuts"`

	// Elevation covers "run as admin" in every form. Off Windows,
	// RunSelfElevatedAndWait is unimplemented and CanKillProcesses always allows,
	// so there is nothing to elevate for.
	Elevation bool `json:"elevation"`

	// ProcessControl covers starting, killing and detecting platform processes -
	// which is most of what a switch does. Implemented on Windows and Linux;
	// winutil/process_other.go (!windows && !linux) stubs it all to
	// ErrUnsupported, so macOS cannot close or relaunch a client at all.
	ProcessControl bool `json:"processControl"`

	// ClosingMethods reports whether choosing between Close/TaskKill/Electron
	// means anything. normalizeClosingMethodForOS collapses every value to
	// "Combined" off Windows, so the picker would offer choices that cannot take
	// effect.
	ClosingMethods bool `json:"closingMethods"`

	// Registry covers reading and writing HKCU/HKLM. winutil/registry_other.go
	// returns ErrUnsupported.
	Registry bool `json:"registry"`

	// ProtocolHandler covers registering the tcno:// URL scheme.
	// winutil/protocol_other.go returns nil without registering anything, which
	// is worse than an error: the toggle reports success and does nothing.
	ProtocolHandler bool `json:"protocolHandler"`

	// BroadcastDetection covers noticing OBS/XSplit for automatic streamer mode.
	// streamer/watch_other.go is an empty setWatching, and nothing off Windows
	// ever calls setDetected, so autoActive can never become true.
	BroadcastDetection bool `json:"broadcastDetection"`

	// ScreenCaptureExclusion covers hiding windows from screen capture. Wails
	// implements setContentProtection on Windows and macOS; its Linux
	// implementation is an empty function body.
	ScreenCaptureExclusion bool `json:"screenCaptureExclusion"`

	// ControllerInput covers gamepad polling. The reader is XInput-only;
	// controllerinput/xinput_stub.go returns nil off Windows and the sync loop
	// then never starts.
	ControllerInput bool `json:"controllerInput"`

	// QRCapture covers scanning the Steam window for a login QR code and picking
	// a screen region. Both are Win32 screen-capture paths.
	QRCapture bool `json:"qrCapture"`

	// SecureClipboard covers writing a Steam Guard code to the clipboard with a
	// timed clear. secureclipboard/platform_other.go returns UnsupportedError.
	SecureClipboard bool `json:"secureClipboard"`

	// SteamBrowser covers the built-in session browser, which is a WebView2 fork
	// with no Linux or macOS backend written yet.
	SteamBrowser bool `json:"steamBrowser"`

	// ServerPicker covers blocking Steam relay clusters, which is implemented
	// only against the Windows Firewall.
	ServerPicker bool `json:"serverPicker"`

	// Autostart covers starting with the session. This one is supported
	// everywhere: Wails writes a ~/.config/autostart entry on Linux and uses
	// SMAppService on macOS.
	Autostart bool `json:"autostart"`
}

// Capabilities reports what this build supports.
func Capabilities() OSCapabilities { return osCapabilities }

// CurrentOS is the GOOS the UI is running against: "windows", "linux" or
// "darwin". Prefer Capabilities; use this only where wording must differ on an
// OS that has the capability.
func CurrentOS() string { return runtime.GOOS }
