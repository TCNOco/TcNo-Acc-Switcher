package qrregion

import (
	"context"
	"errors"
	"runtime"
)

type backend interface {
	selectRegion(context.Context) (Rect, error)
	captureRegion(Rect) (Frame, error)
}

// Selector owns the platform backend for one region-selection request.
type Selector struct{ backend backend }

var selectionSlot = make(chan struct{}, 1)

// New creates a native screen-region selector.
func New() *Selector { return &Selector{backend: newBackend()} }

// Select displays the native overlay and captures the selected region in memory.
// Only one selector may be active in the process at a time.
func (s *Selector) Select(ctx context.Context) (frame Frame, returnErr error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case selectionSlot <- struct{}{}:
		defer func() { <-selectionSlot }()
	default:
		return Frame{}, ErrBusy
	}
	if err := ctx.Err(); err != nil {
		return Frame{}, &CanceledError{Reason: CancelContext}
	}
	if s == nil || s.backend == nil {
		return Frame{}, errors.Join(ErrCapture, errors.New("selector is not initialized"))
	}

	region, err := s.backend.selectRegion(ctx)
	if err != nil {
		return Frame{}, err
	}
	if err := ctx.Err(); err != nil {
		return Frame{}, &CanceledError{Reason: CancelContext}
	}
	if !validCaptureRegion(region) {
		return Frame{}, errors.Join(ErrCapture, errors.New("selected region exceeds capture limits"))
	}
	defer func() {
		if returnErr == nil {
			return
		}
		frame.Wipe()
	}()
	frame, err = s.backend.captureRegion(region)
	if err != nil {
		return Frame{}, err
	}
	if !validFrame(frame, region) {
		return frame, errors.Join(ErrCapture, errors.New("capture backend returned an invalid frame"))
	}
	return frame, nil
}

func validCaptureRegion(region Rect) bool {
	if !region.Valid() {
		return false
	}
	width, height := int64(region.Width()), int64(region.Height())
	return width <= 16384 && height <= 16384 && width*height <= maxCaptureBytes/4
}

func validFrame(frame Frame, region Rect) bool {
	w, h := int64(region.Width()), int64(region.Height())
	expected := w * h * 4
	return frame.Region == region && frame.Width == int(w) && frame.Height == int(h) &&
		frame.Stride == int(w*4) && int64(len(frame.BGRA)) == expected && expected <= maxCaptureBytes
}

func canceled(reason CancelReason) error { return &CanceledError{Reason: reason} }

func wipeBytes(value []byte) {
	clear(value)
	runtime.KeepAlive(value)
}
