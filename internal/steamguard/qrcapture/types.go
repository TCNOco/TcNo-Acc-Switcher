// Package qrcapture discovers verified Steam windows and captures their visible
// pixels for QR decoding. It does not select screen regions or decode images.
package qrcapture

import (
	"errors"
	"fmt"
)

var (
	ErrUnsupported = errors.New("Steam QR capture is unsupported")
	ErrPlatform    = errors.New("Steam QR capture platform failure")
	ErrCapture     = errors.New("Steam QR capture failed")
)

// UnsupportedError identifies a platform without native QR capture support.
type UnsupportedError struct{ GOOS string }

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("Steam QR capture is unsupported on %s", e.GOOS)
}

func (e *UnsupportedError) Unwrap() error { return ErrUnsupported }

// DiscoveryState is safe for a service to map to a user-facing next step.
type DiscoveryState string

const (
	DiscoveryReady         DiscoveryState = "ready"
	DiscoverySteamNotFound DiscoveryState = "steam-not-found"
	DiscoveryNoWindow      DiscoveryState = "no-eligible-window"
	DiscoveryUnsupported   DiscoveryState = "unsupported"
	DiscoveryFailed        DiscoveryState = "failed"
)

// CaptureState reports whether pixels were captured or why the candidate was
// rejected during mandatory capture-time revalidation.
type CaptureState string

const (
	CaptureReady       CaptureState = "ready"
	CaptureUnavailable CaptureState = "window-unavailable"
	CaptureUnsupported CaptureState = "unsupported"
	CaptureFailed      CaptureState = "failed"
)

// Rect uses virtual-screen coordinates and an exclusive right/bottom edge.
type Rect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

func (r Rect) Width() int32  { return r.Right - r.Left }
func (r Rect) Height() int32 { return r.Bottom - r.Top }
func (r Rect) Valid() bool   { return r.Width() > 0 && r.Height() > 0 }

// ProcessInfo is a native process identity snapshot.
type ProcessInfo struct {
	PID            uint32
	ExecutablePath string
}

// WindowInfo is the native state used by the platform-independent filter.
type WindowInfo struct {
	Handle    uintptr
	Owner     uintptr
	PID       uint32
	Title     string
	ClassName string
	Visible   bool
	Cloaked   bool
	Bounds    Rect
}

// Candidate is a verified Steam top-level window and its visible regions. An
// owned dialog remains eligible; Owner is retained for service presentation.
type Candidate struct {
	Handle         uintptr
	Owner          uintptr
	PID            uint32
	Title          string
	ClassName      string
	ExecutablePath string
	SteamRoot      string
	Bounds         Rect
	Regions        []Rect
}

// Frame contains top-down BGRA pixels for one monitor-clipped region.
type Frame struct {
	Region Rect
	Width  int
	Height int
	Stride int
	BGRA   []byte
}

// Discovery is the complete result of one native scan.
type Discovery struct {
	State   DiscoveryState
	Roots   []string
	Windows []Candidate
}

// Capture is a revalidated set of monitor-clipped frames.
type Capture struct {
	State  CaptureState
	Window Candidate
	Frames []Frame
}

// Wipe zeros captured pixels after QR decoding and releases the frame slices.
func (c *Capture) Wipe() {
	for i := range c.Frames {
		for j := range c.Frames[i].BGRA {
			c.Frames[i].BGRA[j] = 0
		}
		c.Frames[i].BGRA = nil
	}
	c.Frames = nil
}

// Backend isolates native discovery and GDI operations for deterministic tests.
type Backend interface {
	RegistrySteamPaths() ([]string, error)
	RunningProcesses() ([]ProcessInfo, error)
	CanonicalSteamRoot(hint string) (string, error)
	CanonicalExecutable(path string) (string, error)
	TopLevelWindows() ([]WindowInfo, error)
	MonitorBounds() ([]Rect, error)
	CaptureRegion(Rect) (Frame, error)
}
