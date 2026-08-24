package platform

// macOS has no process layer yet: winutil/process_other.go is built for
// !windows && !linux and stubs Start, KillByName and every snapshot to
// ErrUnsupported. Until that lands, the app cannot close or relaunch a client,
// which is most of a switch - so ProcessControl is false and the controls that
// depend on it stay hidden rather than failing at the point of use.
var osCapabilities = OSCapabilities{
	Shortcuts:              false,
	Elevation:              false,
	ProcessControl:         false,
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
