package platform

// Windows is the platform every one of these was written against.
var osCapabilities = OSCapabilities{
	Shortcuts:              true,
	Elevation:              true,
	ProcessControl:         true,
	ClosingMethods:         true,
	Registry:               true,
	ProtocolHandler:        true,
	BroadcastDetection:     true,
	ScreenCaptureExclusion: true,
	ControllerInput:        true,
	QRCapture:              true,
	SecureClipboard:        true,
	SteamBrowser:           true,
	ServerPicker:           true,
	Autostart:              true,
}
