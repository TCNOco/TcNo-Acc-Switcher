//go:build windows

package qrregion

import (
	"errors"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	srcCopy      = 0x00CC0020
	captureBlt   = 0x40000000
	biRGB        = 0
	dibRGBColors = 0
)

var (
	procGetDC                  = user32Region.NewProc("GetDC")
	procReleaseDC              = user32Region.NewProc("ReleaseDC")
	procCreateCompatibleDC     = gdi32Region.NewProc("CreateCompatibleDC")
	procDeleteDC               = gdi32Region.NewProc("DeleteDC")
	procCreateCompatibleBitmap = gdi32Region.NewProc("CreateCompatibleBitmap")
	procDeleteObjectCapture    = gdi32Region.NewProc("DeleteObject")
	procSelectObject           = gdi32Region.NewProc("SelectObject")
	procBitBlt                 = gdi32Region.NewProc("BitBlt")
	procGetDIBits              = gdi32Region.NewProc("GetDIBits")
)

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type rgbQuad struct{ Blue, Green, Red, Reserved byte }

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]rgbQuad
}

type captureAPI interface {
	getScreenDC() (uintptr, error)
	releaseScreenDC(uintptr) bool
	createCompatibleDC(uintptr) (uintptr, error)
	deleteDC(uintptr) bool
	createCompatibleBitmap(uintptr, int32, int32) (uintptr, error)
	deleteObject(uintptr) bool
	selectObject(uintptr, uintptr) (uintptr, error)
	bitBlt(uintptr, int32, int32, uintptr, int32, int32) error
	getDIBits(uintptr, uintptr, int32, []byte) error
}

type nativeCaptureAPI struct{}

func (windowsBackend) captureRegion(region Rect) (Frame, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	previousDPI, _, callErr := procSetThreadDpiAwarenessContext.Call(dpiAwarenessPerMonitorV2)
	if previousDPI == 0 {
		return Frame{}, platformError(callErr)
	}
	defer procSetThreadDpiAwarenessContext.Call(previousDPI)
	return capturePhysicalRegion(nativeCaptureAPI{}, region)
}

func capturePhysicalRegion(api captureAPI, region Rect) (frame Frame, returnErr error) {
	defer func() {
		if returnErr == nil {
			return
		}
		frame.Wipe()
	}()
	if !validCaptureRegion(region) {
		return Frame{}, errors.Join(ErrCapture, errors.New("invalid capture bounds"))
	}
	width, height := region.Width(), region.Height()
	screenDC, err := api.getScreenDC()
	if err != nil {
		return Frame{}, err
	}
	defer func() {
		if !api.releaseScreenDC(screenDC) && returnErr == nil {
			returnErr = errors.Join(ErrCapture, errors.New("ReleaseDC failed"))
		}
	}()
	memoryDC, err := api.createCompatibleDC(screenDC)
	if err != nil {
		return Frame{}, err
	}
	defer func() {
		if !api.deleteDC(memoryDC) && returnErr == nil {
			returnErr = errors.Join(ErrCapture, errors.New("DeleteDC failed"))
		}
	}()
	bitmap, err := api.createCompatibleBitmap(screenDC, width, height)
	if err != nil {
		return Frame{}, err
	}
	defer func() {
		if !api.deleteObject(bitmap) && returnErr == nil {
			returnErr = errors.Join(ErrCapture, errors.New("DeleteObject failed"))
		}
	}()
	previous, err := api.selectObject(memoryDC, bitmap)
	if err != nil {
		return Frame{}, err
	}
	selected := true
	defer func() {
		if selected {
			if _, err := api.selectObject(memoryDC, previous); err != nil && returnErr == nil {
				returnErr = errors.Join(ErrCapture, err)
			}
		}
	}()
	if err := api.bitBlt(memoryDC, width, height, screenDC, region.Left, region.Top); err != nil {
		return Frame{}, err
	}
	if _, err := api.selectObject(memoryDC, previous); err != nil {
		return Frame{}, err
	}
	selected = false
	pixels := make([]byte, int(width)*int(height)*4)
	if err := api.getDIBits(memoryDC, bitmap, height, pixels); err != nil {
		wipeBytes(pixels)
		return Frame{}, err
	}
	return Frame{Region: region, Width: int(width), Height: int(height), Stride: int(width) * 4, BGRA: pixels}, nil
}

func (nativeCaptureAPI) getScreenDC() (uintptr, error) {
	handle, _, callErr := procGetDC.Call(0)
	if handle == 0 {
		return 0, platformError(callErr)
	}
	return handle, nil
}

func (nativeCaptureAPI) releaseScreenDC(handle uintptr) bool {
	result, _, _ := procReleaseDC.Call(0, handle)
	return result != 0
}

func (nativeCaptureAPI) createCompatibleDC(source uintptr) (uintptr, error) {
	handle, _, callErr := procCreateCompatibleDC.Call(source)
	if handle == 0 {
		return 0, platformError(callErr)
	}
	return handle, nil
}

func (nativeCaptureAPI) deleteDC(handle uintptr) bool {
	result, _, _ := procDeleteDC.Call(handle)
	return result != 0
}

func (nativeCaptureAPI) createCompatibleBitmap(source uintptr, width, height int32) (uintptr, error) {
	handle, _, callErr := procCreateCompatibleBitmap.Call(source, uintptr(uint32(width)), uintptr(uint32(height)))
	if handle == 0 {
		return 0, platformError(callErr)
	}
	return handle, nil
}

func (nativeCaptureAPI) deleteObject(handle uintptr) bool {
	result, _, _ := procDeleteObjectCapture.Call(handle)
	return result != 0
}

func (nativeCaptureAPI) selectObject(dc, object uintptr) (uintptr, error) {
	previous, _, callErr := procSelectObject.Call(dc, object)
	if previous == 0 || previous == ^uintptr(0) {
		return 0, platformError(callErr)
	}
	return previous, nil
}

func (nativeCaptureAPI) bitBlt(destination uintptr, width, height int32, source uintptr, sourceX, sourceY int32) error {
	result, _, callErr := procBitBlt.Call(
		destination, 0, 0, uintptr(uint32(width)), uintptr(uint32(height)),
		source, uintptr(uint32(sourceX)), uintptr(uint32(sourceY)), srcCopy|captureBlt,
	)
	if result == 0 {
		return platformError(callErr)
	}
	return nil
}

func (nativeCaptureAPI) getDIBits(dc, bitmap uintptr, height int32, pixels []byte) error {
	if len(pixels) == 0 {
		return errors.Join(ErrCapture, errors.New("empty capture buffer"))
	}
	info := bitmapInfo{Header: bitmapInfoHeader{
		Size: uint32(unsafe.Sizeof(bitmapInfoHeader{})), Width: int32(len(pixels) / 4 / int(height)),
		Height: -height, Planes: 1, BitCount: 32, Compression: biRGB, SizeImage: uint32(len(pixels)),
	}}
	lines, _, callErr := procGetDIBits.Call(
		dc, bitmap, 0, uintptr(uint32(height)), uintptr(unsafe.Pointer(&pixels[0])),
		uintptr(unsafe.Pointer(&info)), dibRGBColors,
	)
	runtime.KeepAlive(pixels)
	runtime.KeepAlive(info)
	if lines != uintptr(uint32(height)) {
		if callErr == nil || errors.Is(callErr, syscall.Errno(0)) {
			return ErrCapture
		}
		return errors.Join(ErrCapture, callErr)
	}
	return nil
}
