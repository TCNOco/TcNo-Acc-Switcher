//go:build windows

package secureclipboard

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
	gmemZeroInit  = 0x0040
)

var (
	user32                         = windows.NewLazySystemDLL("user32.dll")
	kernel32                       = windows.NewLazySystemDLL("kernel32.dll")
	procOpenClipboard              = user32.NewProc("OpenClipboard")
	procCloseClipboard             = user32.NewProc("CloseClipboard")
	procCreateWindowExW            = user32.NewProc("CreateWindowExW")
	procDestroyWindow              = user32.NewProc("DestroyWindow")
	procEmptyClipboard             = user32.NewProc("EmptyClipboard")
	procSetClipboardData           = user32.NewProc("SetClipboardData")
	procGetClipboardData           = user32.NewProc("GetClipboardData")
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procGetClipboardSequenceNumber = user32.NewProc("GetClipboardSequenceNumber")
	procRegisterClipboardFormatW   = user32.NewProc("RegisterClipboardFormatW")
	procGlobalAlloc                = kernel32.NewProc("GlobalAlloc")
	procGlobalFree                 = kernel32.NewProc("GlobalFree")
	procGlobalLock                 = kernel32.NewProc("GlobalLock")
	procGlobalUnlock               = kernel32.NewProc("GlobalUnlock")
	procGlobalSize                 = kernel32.NewProc("GlobalSize")
)

type windowsPlatform struct{}

type exclusionFormat struct {
	name  string
	value uint32
}

var exclusionFormats = [...]exclusionFormat{
	{name: "CanIncludeInClipboardHistory", value: 0},
	{name: "CanUploadToCloudClipboard", value: 0},
	{name: "ExcludeClipboardContentFromMonitorProcessing", value: 1},
}

func newPlatform() clipboardPlatform { return windowsPlatform{} }

func (windowsPlatform) write(value code) (writeStamp, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	owner, err := createClipboardOwner()
	if err != nil {
		return writeStamp{}, err
	}
	defer destroyWindow(owner)

	var text [codeLength + 1]uint16
	for i := range value {
		text[i] = uint16(value[i])
	}
	defer wipeUTF16(text[:])

	textHandle, err := allocUTF16(text[:])
	if err != nil {
		return writeStamp{}, err
	}
	textOwned := true
	defer func() {
		if textOwned {
			globalFree(textHandle)
		}
	}()

	if err := openClipboard(owner); err != nil {
		return writeStamp{}, err
	}
	defer closeClipboard()
	if err := emptyClipboard(); err != nil {
		return writeStamp{}, err
	}
	if err := setClipboardData(cfUnicodeText, textHandle); err != nil {
		return writeStamp{}, err
	}
	textOwned = false
	setClipboardExclusions()

	sequence := clipboardSequence()
	if sequence == 0 {
		_ = emptyClipboard()
		return writeStamp{}, ErrUnavailable
	}
	return writeStamp{sequence: sequence, digest: sha256.Sum256(value[:])}, nil
}

func (windowsPlatform) clearIfUnchanged(stamp writeStamp) (bool, error) {
	if err := openClipboard(0); err != nil {
		return false, err
	}
	defer closeClipboard()
	if clipboardSequence() != stamp.sequence {
		return false, nil
	}
	digest, matchesCodeShape, err := currentCodeDigest()
	if err != nil {
		return false, err
	}
	if !matchesCodeShape || subtle.ConstantTimeCompare(digest[:], stamp.digest[:]) != 1 {
		return false, nil
	}
	if err := emptyClipboard(); err != nil {
		return false, err
	}
	return true, nil
}

//go:nocheckptr
func currentCodeDigest() ([sha256.Size]byte, bool, error) {
	var zero [sha256.Size]byte
	available, _, _ := procIsClipboardFormatAvailable.Call(cfUnicodeText)
	if available == 0 {
		return zero, false, nil
	}
	handle, _, callErr := procGetClipboardData.Call(cfUnicodeText)
	if handle == 0 {
		return zero, false, wrapCallError(callErr)
	}
	size, _, callErr := procGlobalSize.Call(handle)
	if size < uintptr((codeLength+1)*2) {
		if size == 0 {
			return zero, false, wrapCallError(callErr)
		}
		return zero, false, nil
	}
	address, _, callErr := procGlobalLock.Call(handle)
	if address == 0 {
		return zero, false, wrapCallError(callErr)
	}
	defer procGlobalUnlock.Call(handle)

	units := unsafe.Slice((*uint16)(unsafe.Add(unsafe.Pointer(nil), address)), codeLength+1)
	var parsed code
	defer parsed.wipe()
	for i := range parsed {
		if units[i] > 0x7f || !isSteamCodeByte(byte(units[i])) {
			return zero, false, nil
		}
		parsed[i] = byte(units[i])
	}
	if units[codeLength] != 0 {
		return zero, false, nil
	}
	runtime.KeepAlive(units)
	return sha256.Sum256(parsed[:]), true, nil
}

func setClipboardExclusions() {
	for _, entry := range exclusionFormats {
		name, err := windows.UTF16PtrFromString(entry.name)
		if err != nil {
			continue
		}
		format, _, _ := procRegisterClipboardFormatW.Call(uintptr(unsafe.Pointer(name)))
		if format == 0 {
			continue
		}
		handle, err := allocDWORD(entry.value)
		if err != nil {
			continue
		}
		if err := setClipboardData(uint32(format), handle); err != nil {
			globalFree(handle)
		}
	}
}

func createClipboardOwner() (uintptr, error) {
	className, err := windows.UTF16PtrFromString("STATIC")
	if err != nil {
		return 0, errors.Join(ErrUnavailable, err)
	}
	window, _, callErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		0,
		0,
		0, 0, 0, 0,
		0, 0, 0, 0,
	)
	if window == 0 {
		return 0, wrapCallError(callErr)
	}
	return window, nil
}

func destroyWindow(window uintptr) {
	if window != 0 {
		procDestroyWindow.Call(window)
	}
}

func openClipboard(owner uintptr) error {
	var last error
	for attempt := 0; attempt < 10; attempt++ {
		result, _, callErr := procOpenClipboard.Call(owner)
		if result != 0 {
			return nil
		}
		last = wrapCallError(callErr)
		time.Sleep(5 * time.Millisecond)
	}
	return last
}

func closeClipboard() {
	procCloseClipboard.Call()
}

func emptyClipboard() error {
	result, _, callErr := procEmptyClipboard.Call()
	if result == 0 {
		return wrapCallError(callErr)
	}
	return nil
}

func clipboardSequence() uint32 {
	sequence, _, _ := procGetClipboardSequenceNumber.Call()
	return uint32(sequence)
}

func setClipboardData(format uint32, handle uintptr) error {
	result, _, callErr := procSetClipboardData.Call(uintptr(format), handle)
	if result == 0 {
		return wrapCallError(callErr)
	}
	return nil
}

//go:nocheckptr
func allocUTF16(value []uint16) (uintptr, error) {
	size := uintptr(len(value) * 2)
	handle, err := globalAlloc(size)
	if err != nil {
		return 0, err
	}
	address, _, callErr := procGlobalLock.Call(handle)
	if address == 0 {
		globalFree(handle)
		return 0, wrapCallError(callErr)
	}
	destination := unsafe.Slice((*uint16)(unsafe.Add(unsafe.Pointer(nil), address)), len(value))
	copy(destination, value)
	procGlobalUnlock.Call(handle)
	runtime.KeepAlive(destination)
	return handle, nil
}

//go:nocheckptr
func allocDWORD(value uint32) (uintptr, error) {
	handle, err := globalAlloc(4)
	if err != nil {
		return 0, err
	}
	address, _, callErr := procGlobalLock.Call(handle)
	if address == 0 {
		globalFree(handle)
		return 0, wrapCallError(callErr)
	}
	*(*uint32)(unsafe.Add(unsafe.Pointer(nil), address)) = value
	procGlobalUnlock.Call(handle)
	return handle, nil
}

func globalAlloc(size uintptr) (uintptr, error) {
	handle, _, callErr := procGlobalAlloc.Call(gmemMoveable|gmemZeroInit, size)
	if handle == 0 {
		return 0, wrapCallError(callErr)
	}
	return handle, nil
}

func globalFree(handle uintptr) {
	if handle != 0 {
		procGlobalFree.Call(handle)
	}
}

func wrapCallError(callErr error) error {
	if callErr == nil || errors.Is(callErr, syscall.Errno(0)) {
		return ErrUnavailable
	}
	return errors.Join(ErrUnavailable, callErr)
}

func wipeUTF16(value []uint16) {
	for i := range value {
		value[i] = 0
	}
	runtime.KeepAlive(value)
}
