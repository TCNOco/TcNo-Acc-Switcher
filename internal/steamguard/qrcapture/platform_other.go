//go:build !windows

package qrcapture

import "runtime"

type unsupportedBackend struct{}

func newBackend() Backend { return unsupportedBackend{} }

func unsupported() error { return &UnsupportedError{GOOS: runtime.GOOS} }

func (unsupportedBackend) RegistrySteamPaths() ([]string, error)      { return nil, unsupported() }
func (unsupportedBackend) RunningProcesses() ([]ProcessInfo, error)   { return nil, unsupported() }
func (unsupportedBackend) CanonicalSteamRoot(string) (string, error)  { return "", unsupported() }
func (unsupportedBackend) CanonicalExecutable(string) (string, error) { return "", unsupported() }
func (unsupportedBackend) TopLevelWindows() ([]WindowInfo, error)     { return nil, unsupported() }
func (unsupportedBackend) MonitorBounds() ([]Rect, error)             { return nil, unsupported() }
func (unsupportedBackend) CaptureRegion(Rect) (Frame, error)          { return Frame{}, unsupported() }
