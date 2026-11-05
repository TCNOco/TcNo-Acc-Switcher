//go:build windows

package securemem

import (
	"errors"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	cryptProtectSameProcess = 0x00
	cryptProtectBlockSize   = 16
)

var (
	crypt32             = windows.NewLazySystemDLL("crypt32.dll")
	procProtectMemory   = crypt32.NewProc("CryptProtectMemory")
	procUnprotectMemory = crypt32.NewProc("CryptUnprotectMemory")
)

type windowsProtector struct{}

type windowsHandle struct {
	mu        sync.Mutex
	address   uintptr
	buffer    []byte
	length    int
	destroyed bool
}

func newPlatformProtector() Protector { return windowsProtector{} }

func (windowsProtector) Store(secret []byte) (Handle, error) {
	if len(secret) == 0 {
		return nil, ErrUnavailable
	}
	paddedLength := (len(secret) + cryptProtectBlockSize - 1) &^ (cryptProtectBlockSize - 1)
	if paddedLength < len(secret) {
		return nil, ErrUnavailable
	}
	address, err := windows.VirtualAlloc(0, uintptr(paddedLength), windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
	if err != nil {
		return nil, errors.Join(ErrUnavailable, err)
	}
	buf := virtualBytes(address, paddedLength)
	copy(buf, secret)
	if err := windows.VirtualLock(address, uintptr(len(buf))); err != nil {
		wipe(buf)
		_ = windows.VirtualFree(address, 0, windows.MEM_RELEASE)
		return nil, errors.Join(ErrUnavailable, err)
	}
	h := &windowsHandle{address: address, buffer: buf, length: len(secret)}
	if err := h.protect(); err != nil {
		_ = windows.VirtualUnlock(address, uintptr(len(buf)))
		wipe(buf)
		_ = windows.VirtualFree(address, 0, windows.MEM_RELEASE)
		return nil, errors.Join(ErrUnavailable, err)
	}
	return h, nil
}

//go:nocheckptr
func virtualBytes(address uintptr, length int) []byte {
	return unsafe.Slice((*byte)(unsafe.Add(unsafe.Pointer(nil), address)), length)
}

func (h *windowsHandle) protect() error {
	r1, _, callErr := procProtectMemory.Call(h.address, uintptr(len(h.buffer)), cryptProtectSameProcess)
	runtime.KeepAlive(h.buffer)
	if r1 == 0 {
		return callErr
	}
	return nil
}

func (h *windowsHandle) unprotect() error {
	r1, _, callErr := procUnprotectMemory.Call(h.address, uintptr(len(h.buffer)), cryptProtectSameProcess)
	runtime.KeepAlive(h.buffer)
	if r1 == 0 {
		return callErr
	}
	return nil
}

func (h *windowsHandle) With(fn func([]byte) error) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.destroyed || len(h.buffer) == 0 {
		return ErrUnavailable
	}
	if err := h.unprotect(); err != nil {
		return errors.Join(ErrUnavailable, err)
	}
	copyOfSecret := append([]byte(nil), h.buffer[:h.length]...)
	protectErr := h.protect()
	if protectErr != nil {
		wipe(h.buffer)
		_ = windows.VirtualUnlock(h.address, uintptr(len(h.buffer)))
		_ = windows.VirtualFree(h.address, 0, windows.MEM_RELEASE)
		h.address = 0
		h.buffer = nil
		h.length = 0
		h.destroyed = true
	}
	defer func() {
		wipe(copyOfSecret)
		runtime.KeepAlive(copyOfSecret)
	}()
	if protectErr != nil {
		return errors.Join(ErrUnavailable, protectErr)
	}
	return fn(copyOfSecret)
}

func (h *windowsHandle) Destroy() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.destroyed || len(h.buffer) == 0 {
		return nil
	}
	unprotectErr := h.unprotect()
	wipe(h.buffer)
	unlockErr := windows.VirtualUnlock(h.address, uintptr(len(h.buffer)))
	freeErr := windows.VirtualFree(h.address, 0, windows.MEM_RELEASE)
	runtime.KeepAlive(h.buffer)
	h.address = 0
	h.buffer = nil
	h.length = 0
	h.destroyed = true
	if unprotectErr != nil || unlockErr != nil || freeErr != nil {
		return errors.Join(ErrUnavailable, unprotectErr, unlockErr, freeErr)
	}
	return nil
}
