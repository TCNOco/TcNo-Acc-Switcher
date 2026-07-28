//go:build windows

package qrcapture

import (
	"errors"
	"reflect"
	"testing"
)

type fakeGDI struct {
	calls    []string
	failAt   string
	selected uintptr
	pixels   []byte
}

func (f *fakeGDI) call(name string) error {
	f.calls = append(f.calls, name)
	if f.failAt == name {
		return errors.New("injected " + name)
	}
	return nil
}
func (f *fakeGDI) getScreenDC() (uintptr, error) {
	if err := f.call("get-screen"); err != nil {
		return 0, err
	}
	return 1, nil
}
func (f *fakeGDI) releaseScreenDC(uintptr) bool { return f.call("release-screen") == nil }
func (f *fakeGDI) createCompatibleDC(uintptr) (uintptr, error) {
	if err := f.call("create-dc"); err != nil {
		return 0, err
	}
	return 2, nil
}
func (f *fakeGDI) deleteDC(uintptr) bool { return f.call("delete-dc") == nil }
func (f *fakeGDI) createCompatibleBitmap(uintptr, int32, int32) (uintptr, error) {
	if err := f.call("create-bitmap"); err != nil {
		return 0, err
	}
	return 3, nil
}
func (f *fakeGDI) deleteObject(uintptr) bool { return f.call("delete-bitmap") == nil }
func (f *fakeGDI) selectObject(_ uintptr, object uintptr) (uintptr, error) {
	name := "select-bitmap"
	if object == 4 {
		name = "restore-object"
	}
	if err := f.call(name); err != nil {
		return 0, err
	}
	if object == 3 {
		f.selected = 3
		return 4, nil
	}
	f.selected = object
	return 3, nil
}
func (f *fakeGDI) bitBlt(uintptr, int32, int32, uintptr, int32, int32) error {
	return f.call("bitblt")
}
func (f *fakeGDI) getDIBits(_ uintptr, _ uintptr, _ int32, pixels []byte) error {
	if err := f.call("get-dibits"); err != nil {
		return err
	}
	for i := range pixels {
		pixels[i] = byte(i)
	}
	f.pixels = pixels
	return nil
}

func TestCaptureRegionReleasesEveryResource(t *testing.T) {
	api := &fakeGDI{}
	frame, err := captureRegion(api, Rect{Left: -10, Top: 20, Right: 90, Bottom: 70})
	if err != nil {
		t.Fatal(err)
	}
	if frame.Width != 100 || frame.Height != 50 || len(frame.BGRA) != 20_000 {
		t.Fatalf("unexpected frame: %#v", frame)
	}
	want := []string{"get-screen", "create-dc", "create-bitmap", "select-bitmap", "bitblt", "restore-object", "get-dibits", "delete-bitmap", "delete-dc", "release-screen"}
	if !reflect.DeepEqual(api.calls, want) {
		t.Fatalf("calls = %v, want %v", api.calls, want)
	}
}

func TestCaptureRegionCleansUpAfterEachFailure(t *testing.T) {
	tests := []struct {
		fail string
		want []string
	}{
		{"create-dc", []string{"get-screen", "create-dc", "release-screen"}},
		{"create-bitmap", []string{"get-screen", "create-dc", "create-bitmap", "delete-dc", "release-screen"}},
		{"select-bitmap", []string{"get-screen", "create-dc", "create-bitmap", "select-bitmap", "delete-bitmap", "delete-dc", "release-screen"}},
		{"bitblt", []string{"get-screen", "create-dc", "create-bitmap", "select-bitmap", "bitblt", "restore-object", "delete-bitmap", "delete-dc", "release-screen"}},
		{"get-dibits", []string{"get-screen", "create-dc", "create-bitmap", "select-bitmap", "bitblt", "restore-object", "get-dibits", "delete-bitmap", "delete-dc", "release-screen"}},
	}
	for _, test := range tests {
		t.Run(test.fail, func(t *testing.T) {
			api := &fakeGDI{failAt: test.fail}
			if _, err := captureRegion(api, Rect{Right: 10, Bottom: 10}); err == nil {
				t.Fatal("expected error")
			}
			if !reflect.DeepEqual(api.calls, test.want) {
				t.Fatalf("calls = %v, want %v", api.calls, test.want)
			}
		})
	}
}

func TestCaptureRegionWipesPixelsWhenDeferredCleanupFails(t *testing.T) {
	for _, failure := range []string{"delete-bitmap", "delete-dc", "release-screen"} {
		t.Run(failure, func(t *testing.T) {
			api := &fakeGDI{failAt: failure}
			frame, err := captureRegion(api, Rect{Right: 10, Bottom: 10})
			if err == nil {
				t.Fatal("expected cleanup error")
			}
			if frame.BGRA != nil || frame.Width != 0 || frame.Height != 0 {
				t.Fatalf("capture returned pixels with cleanup error: %#v", frame)
			}
			for i, value := range api.pixels {
				if value != 0 {
					t.Fatalf("pixel %d was not wiped", i)
				}
			}
		})
	}
}
