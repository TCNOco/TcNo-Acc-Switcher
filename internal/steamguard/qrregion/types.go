// Package qrregion provides a native, ephemeral screen-region selector for QR
// decoding. Captured pixels are returned in memory and are never written to disk.
package qrregion

import (
	"errors"
	"fmt"
	"runtime"
)

const maxCaptureBytes = 64 << 20

var (
	ErrUnsupported = errors.New("Steam QR region selection is unsupported")
	ErrCanceled    = errors.New("Steam QR region selection was canceled")
	ErrBusy        = errors.New("another Steam QR region selection is active")
	ErrCapture     = errors.New("Steam QR region capture failed")
)

// CancelReason identifies why the selector closed without capturing pixels.
type CancelReason string

const (
	CancelEscape        CancelReason = "escape"
	CancelRightClick    CancelReason = "right-click"
	CancelFocusLost     CancelReason = "focus-lost"
	CancelDisplayChange CancelReason = "display-change"
	CancelContext       CancelReason = "context-canceled"
	CancelEmptyRegion   CancelReason = "empty-region"
)

// CanceledError is safe to map to a user-facing cancellation state.
type CanceledError struct{ Reason CancelReason }

func (e *CanceledError) Error() string {
	return fmt.Sprintf("Steam QR region selection canceled: %s", e.Reason)
}

func (e *CanceledError) Unwrap() error { return ErrCanceled }

// UnsupportedError identifies a platform without a native selector.
type UnsupportedError struct{ GOOS string }

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("Steam QR region selection is unsupported on %s", e.GOOS)
}

func (e *UnsupportedError) Unwrap() error { return ErrUnsupported }

// Rect uses physical virtual-screen coordinates with exclusive right/bottom edges.
type Rect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

func (r Rect) Width() int32  { return r.Right - r.Left }
func (r Rect) Height() int32 { return r.Bottom - r.Top }
func (r Rect) Valid() bool   { return r.Width() > 0 && r.Height() > 0 }

// Frame contains top-down BGRA pixels from the selected physical screen region.
type Frame struct {
	Region Rect
	Width  int
	Height int
	Stride int
	BGRA   []byte
}

// Wipe zeros captured pixels and releases the backing slice.
func (f *Frame) Wipe() {
	clear(f.BGRA)
	runtime.KeepAlive(f.BGRA)
	f.BGRA = nil
	f.Width = 0
	f.Height = 0
	f.Stride = 0
}
