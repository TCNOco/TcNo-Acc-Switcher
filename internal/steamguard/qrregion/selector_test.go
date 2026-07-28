package qrregion

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fakeBackend struct {
	region       Rect
	selectErr    error
	captureErr   error
	frame        Frame
	calls        []string
	capturedWith Rect
}

func (f *fakeBackend) selectRegion(context.Context) (Rect, error) {
	f.calls = append(f.calls, "select-and-hide")
	return f.region, f.selectErr
}

func (f *fakeBackend) captureRegion(region Rect) (Frame, error) {
	f.calls = append(f.calls, "capture")
	f.capturedWith = region
	return f.frame, f.captureErr
}

func TestSelectCapturesOnlyAfterSelectionOverlayHides(t *testing.T) {
	region := Rect{Left: -30, Top: 40, Right: 70, Bottom: 90}
	pixels := make([]byte, 100*50*4)
	fake := &fakeBackend{
		region: region,
		frame:  Frame{Region: region, Width: 100, Height: 50, Stride: 400, BGRA: pixels},
	}
	frame, err := (&Selector{backend: fake}).Select(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer frame.Wipe()
	if !reflect.DeepEqual(fake.calls, []string{"select-and-hide", "capture"}) {
		t.Fatalf("calls = %v", fake.calls)
	}
	if fake.capturedWith != region || len(frame.BGRA) != len(pixels) {
		t.Fatalf("unexpected capture: %#v", frame)
	}
}

func TestSelectDoesNotCaptureAfterCancellation(t *testing.T) {
	fake := &fakeBackend{selectErr: canceled(CancelEscape)}
	_, err := (&Selector{backend: fake}).Select(context.Background())
	var canceledErr *CanceledError
	if !errors.As(err, &canceledErr) || canceledErr.Reason != CancelEscape {
		t.Fatalf("error = %v", err)
	}
	if !reflect.DeepEqual(fake.calls, []string{"select-and-hide"}) {
		t.Fatalf("calls = %v", fake.calls)
	}
}

func TestSelectRejectsOversizedRegionBeforeCapture(t *testing.T) {
	fake := &fakeBackend{region: Rect{Right: 16385, Bottom: 1}}
	_, err := (&Selector{backend: fake}).Select(context.Background())
	if !errors.Is(err, ErrCapture) {
		t.Fatalf("error = %v", err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("capture called for oversized selection: %v", fake.calls)
	}
}

func TestSelectWipesInvalidBackendFrame(t *testing.T) {
	region := Rect{Right: 2, Bottom: 2}
	pixels := []byte{1, 2, 3, 4}
	fake := &fakeBackend{
		region: region,
		frame:  Frame{Region: region, Width: 1, Height: 1, Stride: 4, BGRA: pixels},
	}
	frame, err := (&Selector{backend: fake}).Select(context.Background())
	if !errors.Is(err, ErrCapture) {
		t.Fatalf("error = %v", err)
	}
	if frame.BGRA != nil {
		t.Fatal("invalid frame was returned")
	}
	for i, value := range pixels {
		if value != 0 {
			t.Fatalf("pixel %d was not wiped", i)
		}
	}
}

func TestSelectMapsPreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (&Selector{backend: &fakeBackend{}}).Select(ctx)
	var canceledErr *CanceledError
	if !errors.As(err, &canceledErr) || canceledErr.Reason != CancelContext {
		t.Fatalf("error = %v", err)
	}
}

func TestSelectRejectsConcurrentOverlay(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	region := Rect{Right: 1, Bottom: 1}
	blocking := &blockingBackend{entered: entered, release: release, region: region}
	done := make(chan error, 1)
	go func() {
		frame, err := (&Selector{backend: blocking}).Select(context.Background())
		frame.Wipe()
		done <- err
	}()
	<-entered
	_, err := (&Selector{backend: &fakeBackend{}}).Select(context.Background())
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("error = %v, want ErrBusy", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

type blockingBackend struct {
	entered chan struct{}
	release chan struct{}
	region  Rect
}

func (b *blockingBackend) selectRegion(context.Context) (Rect, error) {
	close(b.entered)
	<-b.release
	return b.region, nil
}

func (b *blockingBackend) captureRegion(region Rect) (Frame, error) {
	return Frame{Region: region, Width: 1, Height: 1, Stride: 4, BGRA: make([]byte, 4)}, nil
}

func TestFrameWipeClearsPixels(t *testing.T) {
	pixels := []byte{1, 2, 3, 4}
	frame := Frame{Width: 1, Height: 1, Stride: 4, BGRA: pixels}
	frame.Wipe()
	if frame.BGRA != nil || frame.Width != 0 || frame.Height != 0 || frame.Stride != 0 {
		t.Fatalf("frame was not released: %#v", frame)
	}
	for i, value := range pixels {
		if value != 0 {
			t.Fatalf("pixel %d was not wiped", i)
		}
	}
}
