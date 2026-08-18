//go:build windows

package webview2

import (
	"sync/atomic"
	"testing"
)

// eAbort is what WebView2 completes controller creation with when the host
// window is destroyed before it finishes.
const eAbort = 0x80004004

// A failed creation must not reach initializeController. It is handed no
// controller to initialise, so the old fall-through dereferenced nil - which
// only ever went unnoticed because the error path exited the process first.
//
// The zero value is enough to exercise this: no window, no runtime, no COM.
func TestControllerCreationFailureDoesNotInitialise(t *testing.T) {
	e := &Chromium{}

	if got := e.CreateCoreWebView2ControllerCompleted(eAbort, nil); got != 0 {
		t.Errorf("completed handler returned %#x, want 0", got)
	}
	if atomic.LoadUintptr(&e.createFailed) == 0 {
		t.Error("createFailed not set, so Embed's pump would never wake")
	}
	if e.IsReady() {
		t.Error("IsReady reports a controller that was never created")
	}
	if e.controller != nil || e.webview != nil {
		t.Error("a failed creation left a controller or view behind")
	}
}

// Success is reported as res >= 0, so a nil controller alongside it still has to
// be refused rather than initialised.
func TestControllerCreationRefusesNilOnSuccess(t *testing.T) {
	e := &Chromium{}

	if got := e.CreateCoreWebView2ControllerCompleted(0, nil); got != 0 {
		t.Errorf("completed handler returned %#x, want 0", got)
	}
	if e.IsReady() {
		t.Error("IsReady reports ready after being handed no controller")
	}
}
