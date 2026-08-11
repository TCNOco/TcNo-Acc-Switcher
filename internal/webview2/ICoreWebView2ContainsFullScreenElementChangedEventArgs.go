//go:build windows

package webview2

import (
	"unsafe"
)

type _ICoreWebView2ContainsFullScreenElementChangedEventArgsVtbl struct {
	_IUnknownVtbl
}

type ICoreWebView2ContainsFullScreenElementChangedEventArgs struct {
	vtbl *_ICoreWebView2ContainsFullScreenElementChangedEventArgsVtbl
}

func (i *ICoreWebView2ContainsFullScreenElementChangedEventArgs) AddRef() uintptr {
	ret, _, _ := i.vtbl.AddRef.Call(uintptr(unsafe.Pointer(i)))

	return ret
}
