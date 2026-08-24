package platform

// Linux has the process layer - winutil/process_linux.go implements start, the
// SIGTERM/SIGKILL ladder and /proc snapshots - so switching works. What is
// missing is the Win32-specific decoration around it.
//
// ScreenCaptureExclusion is false here and true on macOS: Wails implements
// setContentProtection for NSWindow but leaves the GTK one an empty function.
var osCapabilities = OSCapabilities{
	Shortcuts:              false,
	Elevation:              false,
	ProcessControl:         true,
	ClosingMethods:         false,
	Registry:               false,
	ProtocolHandler:        false,
	BroadcastDetection:     false,
	ScreenCaptureExclusion: false,
	ControllerInput:        false,
	QRCapture:              false,
	SecureClipboard:        false,
	SteamBrowser:           false,
	ServerPicker:           false,
	Autostart:              true,
}
