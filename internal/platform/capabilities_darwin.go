package platform

// macOS shares the POSIX process layer with Linux - winutil/process_unix.go -
// and differs only in how the process table is read, which is a kern.proc.all
// sysctl here rather than a /proc walk. So closing and relaunching a client
// works, and with it the switch itself.
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
