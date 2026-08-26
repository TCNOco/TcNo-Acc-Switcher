package platform

// macOS shares the POSIX process layer with Linux (winutil/process_unix.go),
// reading the process table through a kern.proc.all sysctl rather than /proc, so
// closing and relaunching a client works.
//
// ScreenCaptureExclusion is true here and false on Linux: Wails implements
// setContentProtection for NSWindow and leaves the GTK one an empty function.
var osCapabilities = OSCapabilities{
	Shortcuts:              false,
	Elevation:              false,
	ProcessControl:         true,
	ClosingMethods:         false,
	Registry:               false,
	ProtocolHandler:        false,
	BroadcastDetection:     false,
	ScreenCaptureExclusion: true,
	ControllerInput:        false,
	QRCapture:              false,
	SecureClipboard:        false,
	SteamBrowser:           false,
	ServerPicker:           false,
	Autostart:              true,
}
