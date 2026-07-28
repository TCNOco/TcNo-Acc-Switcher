//go:build windows

package qrcapture

import (
	"errors"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	srccopy         = 0x00CC0020
	captureblt      = 0x40000000
	biRGB           = 0
	dibRGBColors    = 0
	maxCaptureBytes = 64 << 20
)

var (
	gdi32QR                    = windows.NewLazySystemDLL("gdi32.dll")
	procGetDC                  = user32QR.NewProc("GetDC")
	procReleaseDC              = user32QR.NewProc("ReleaseDC")
	procCreateCompatibleDC     = gdi32QR.NewProc("CreateCompatibleDC")
	procDeleteDC               = gdi32QR.NewProc("DeleteDC")
	procCreateCompatibleBitmap = gdi32QR.NewProc("CreateCompatibleBitmap")
	procDeleteObject           = gdi32QR.NewProc("DeleteObject")
	procSelectObject           = gdi32QR.NewProc("SelectObject")
	procBitBlt                 = gdi32QR.NewProc("BitBlt")
	procGetDIBits              = gdi32QR.NewProc("GetDIBits")
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

type rgbQuad struct {
	Blue     byte
	Green    byte
	Red      byte
	Reserved byte
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]rgbQuad
}

type gdiAPI interface {
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

type nativeGDI struct{}

func (windowsBackend) CaptureRegion(region Rect) (Frame, error) {
	return captureRegion(nativeGDI{}, region)
}

func captureRegion(api gdiAPI, region Rect) (frame Frame, returnErr error) {
	defer func() {
		if returnErr == nil {
			return
		}
		pixels := frame.BGRA
		clear(pixels)
		runtime.KeepAlive(pixels)
		frame = Frame{}
	}()
	if !region.Valid() {
		return Frame{}, errors.Join(ErrCapture, errors.New("invalid capture bounds"))
	}
	width, height := int64(region.Width()), int64(region.Height())
	if width > 16384 || height > 16384 || width*height > maxCaptureBytes/4 {
		return Frame{}, errors.Join(ErrCapture, errors.New("capture bounds exceed limit"))
	}

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
	bitmap, err := api.createCompatibleBitmap(screenDC, int32(width), int32(height))
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
	bitmapSelected := true
	defer func() {
		if bitmapSelected {
			_, err := api.selectObject(memoryDC, previous)
			if err != nil && returnErr == nil {
				returnErr = errors.Join(ErrCapture, err)
			}
		}
	}()
	if err := api.bitBlt(memoryDC, int32(width), int32(height), screenDC, region.Left, region.Top); err != nil {
		return Frame{}, err
	}
	if _, err := api.selectObject(memoryDC, previous); err != nil {
		return Frame{}, err
	}
	bitmapSelected = false
	pixels := make([]byte, int(width*height*4))
	if err := api.getDIBits(memoryDC, bitmap, int32(height), pixels); err != nil {
		return Frame{}, err
	}
	return Frame{Region: region, Width: int(width), Height: int(height), Stride: int(width * 4), BGRA: pixels}, nil
}

func (nativeGDI) getScreenDC() (uintptr, error) {
	handle, _, callErr := procGetDC.Call(0)
	if handle == 0 {
		return 0, wrapWindowsCall(callErr)
	}
	return handle, nil
}

func (nativeGDI) releaseScreenDC(handle uintptr) bool {
	result, _, _ := procReleaseDC.Call(0, handle)
	return result != 0
}

func (nativeGDI) createCompatibleDC(source uintptr) (uintptr, error) {
	handle, _, callErr := procCreateCompatibleDC.Call(source)
	if handle == 0 {
		return 0, wrapWindowsCall(callErr)
	}
	return handle, nil
}

func (nativeGDI) deleteDC(handle uintptr) bool {
	result, _, _ := procDeleteDC.Call(handle)
	return result != 0
}

func (nativeGDI) createCompatibleBitmap(source uintptr, width, height int32) (uintptr, error) {
	handle, _, callErr := procCreateCompatibleBitmap.Call(source, uintptr(uint32(width)), uintptr(uint32(height)))
	if handle == 0 {
		return 0, wrapWindowsCall(callErr)
	}
	return handle, nil
}

func (nativeGDI) deleteObject(handle uintptr) bool {
	result, _, _ := procDeleteObject.Call(handle)
	return result != 0
}

func (nativeGDI) selectObject(dc, object uintptr) (uintptr, error) {
	previous, _, callErr := procSelectObject.Call(dc, object)
	if previous == 0 || previous == ^uintptr(0) {
		return 0, wrapWindowsCall(callErr)
	}
	return previous, nil
}

func (nativeGDI) bitBlt(destination uintptr, width, height int32, source uintptr, sourceX, sourceY int32) error {
	result, _, callErr := procBitBlt.Call(
		destination, 0, 0, uintptr(uint32(width)), uintptr(uint32(height)),
		source, uintptr(uint32(sourceX)), uintptr(uint32(sourceY)), srccopy|captureblt,
	)
	if result == 0 {
		return wrapWindowsCall(callErr)
	}
	return nil
}

func (nativeGDI) getDIBits(dc, bitmap uintptr, height int32, pixels []byte) error {
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
