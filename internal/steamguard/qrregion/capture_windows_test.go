//go:build windows

package qrregion

import (
	"errors"
	"reflect"
	"testing"
)

type fakeCaptureAPI struct {
	calls  []string
	failAt string
	pixels []byte
}

func (f *fakeCaptureAPI) call(name string) error {
	f.calls = append(f.calls, name)
	if name == f.failAt {
		return errors.New("injected " + name)
	}
	return nil
}

func (f *fakeCaptureAPI) getScreenDC() (uintptr, error) {
	if err := f.call("get-screen"); err != nil {
		return 0, err
	}
	return 1, nil
}
func (f *fakeCaptureAPI) releaseScreenDC(uintptr) bool { return f.call("release-screen") == nil }
func (f *fakeCaptureAPI) createCompatibleDC(uintptr) (uintptr, error) {
	if err := f.call("create-dc"); err != nil {
		return 0, err
	}
	return 2, nil
}
func (f *fakeCaptureAPI) deleteDC(uintptr) bool { return f.call("delete-dc") == nil }
func (f *fakeCaptureAPI) createCompatibleBitmap(uintptr, int32, int32) (uintptr, error) {
	if err := f.call("create-bitmap"); err != nil {
		return 0, err
	}
	return 3, nil
}
func (f *fakeCaptureAPI) deleteObject(uintptr) bool { return f.call("delete-bitmap") == nil }
func (f *fakeCaptureAPI) selectObject(_ uintptr, object uintptr) (uintptr, error) {
	name := "select-bitmap"
	if object == 4 {
		name = "restore-object"
	}
	if err := f.call(name); err != nil {
		return 0, err
	}
	if object == 3 {
		return 4, nil
	}
	return 3, nil
}
func (f *fakeCaptureAPI) bitBlt(uintptr, int32, int32, uintptr, int32, int32) error {
	return f.call("bitblt")
}
func (f *fakeCaptureAPI) getDIBits(_ uintptr, _ uintptr, _ int32, pixels []byte) error {
	if err := f.call("get-dibits"); err != nil {
		return err
	}
	for i := range pixels {
		pixels[i] = byte(i + 1)
	}
	f.pixels = pixels
	return nil
}

func TestCapturePhysicalRegionReleasesResources(t *testing.T) {
	api := &fakeCaptureAPI{}
	frame, err := capturePhysicalRegion(api, Rect{Left: -20, Top: 10, Right: 80, Bottom: 60})
	if err != nil {
		t.Fatal(err)
	}
	defer frame.Wipe()
	want := []string{"get-screen", "create-dc", "create-bitmap", "select-bitmap", "bitblt", "restore-object", "get-dibits", "delete-bitmap", "delete-dc", "release-screen"}
	if !reflect.DeepEqual(api.calls, want) {
		t.Fatalf("calls = %v, want %v", api.calls, want)
	}
}

func TestCapturePhysicalRegionCleansUpAndWipesOnFailures(t *testing.T) {
	for _, failure := range []string{"create-dc", "create-bitmap", "select-bitmap", "bitblt", "get-dibits", "delete-bitmap", "delete-dc", "release-screen"} {
		t.Run(failure, func(t *testing.T) {
			api := &fakeCaptureAPI{failAt: failure}
			frame, err := capturePhysicalRegion(api, Rect{Right: 10, Bottom: 10})
			if err == nil {
				t.Fatal("expected error")
			}
			if frame.BGRA != nil {
				t.Fatal("pixels returned after failure")
			}
			for i, value := range api.pixels {
				if value != 0 {
					t.Fatalf("pixel %d was not wiped", i)
				}
			}
		})
	}
}
