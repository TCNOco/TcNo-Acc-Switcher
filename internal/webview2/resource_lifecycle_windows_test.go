//go:build windows

package webview2

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestDeferralReferenceCountingUsesCorrectCOMSlots(t *testing.T) {
	adds, releases := 0, 0
	deferral := &ICoreWebView2Deferral{Vtbl: &ICoreWebView2DeferralVtbl{
		_IUnknownVtbl: _IUnknownVtbl{
			AddRef:  ComProc(windows.NewCallback(func(uintptr) uintptr { adds++; return 2 })),
			Release: ComProc(windows.NewCallback(func(uintptr) uintptr { releases++; return 1 })),
		},
	}}
	if got := deferral.AddRef(); got != 2 {
		t.Fatalf("AddRef count = %d", got)
	}
	if got := deferral.Release(); got != 1 || adds != 1 || releases != 1 {
		t.Fatalf("Release count=%d adds=%d releases=%d", got, adds, releases)
	}
}

func TestResourceRequestFailureDoesNotExitOrDispatch(t *testing.T) {
	view := &Chromium{WebResourceRequestedCallback: func(*ICoreWebView2WebResourceRequest, *ICoreWebView2WebResourceRequestedEventArgs) {
		t.Fatal("unavailable request reached the handler")
	}}
	view.WebResourceRequested(nil, nil)
	for _, status := range []uintptr{0, 0x80004005} {
		args := &ICoreWebView2WebResourceRequestedEventArgs{vtbl: &_ICoreWebView2WebResourceRequestedEventArgsVtbl{
			GetRequest: ComProc(windows.NewCallback(func(uintptr, uintptr) uintptr { return status })),
		}}
		view.WebResourceRequested(nil, args)
	}
}

func TestResourceRequestReleasesReferenceAfterDispatch(t *testing.T) {
	releases, calls := 0, 0
	request := &ICoreWebView2WebResourceRequest{vtbl: &_ICoreWebView2WebResourceRequestVtbl{
		_IUnknownVtbl: _IUnknownVtbl{Release: ComProc(windows.NewCallback(func(uintptr) uintptr { releases++; return 0 }))},
	}}
	args := &ICoreWebView2WebResourceRequestedEventArgs{vtbl: &_ICoreWebView2WebResourceRequestedEventArgsVtbl{
		GetRequest: ComProc(windows.NewCallback(func(_ uintptr, out **ICoreWebView2WebResourceRequest) uintptr { *out = request; return 0 })),
	}}
	view := &Chromium{WebResourceRequestedCallback: func(got *ICoreWebView2WebResourceRequest, _ *ICoreWebView2WebResourceRequestedEventArgs) {
		calls++
		if unsafe.Pointer(got) != unsafe.Pointer(request) {
			t.Fatal("wrong COM request dispatched")
		}
	}}
	view.WebResourceRequested(nil, args)
	if calls != 1 || releases != 1 {
		t.Fatalf("dispatches=%d releases=%d", calls, releases)
	}
}
