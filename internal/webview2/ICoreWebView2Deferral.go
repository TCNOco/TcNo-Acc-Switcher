//go:build windows

package webview2

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type ICoreWebView2DeferralVtbl struct {
	_IUnknownVtbl
	Complete ComProc
}

type ICoreWebView2Deferral struct {
	Vtbl *ICoreWebView2DeferralVtbl
}

// AddRef increments the reference count of ICoreWebView2Deferral interface
func (i *ICoreWebView2Deferral) AddRef() uint32 {
	// IUnknown returns a reference count, not HRESULT or GetLastError.
	count, _, _ := i.Vtbl.AddRef.Call(uintptr(unsafe.Pointer(i)))
	return uint32(count)
}

// Release decrements the reference count of ICoreWebView2Deferral interface
func (i *ICoreWebView2Deferral) Release() uint32 {
	count, _, _ := i.Vtbl.Release.Call(uintptr(unsafe.Pointer(i)))
	return uint32(count)
}

func (i *ICoreWebView2Deferral) Complete() error {
	hr, _, _ := i.Vtbl.Complete.Call(
		uintptr(unsafe.Pointer(i)),
	)
	if windows.Handle(hr) != windows.S_OK {
		return syscall.Errno(hr)
	}
	return nil
}
