//go:build windows

package webview2

// Accessors a host outside this package needs. Chromium keeps its webview and
// controller unexported, which is fine for Wails, whose window owns both; a
// caller driving its own toolbar has to reach them.

// GetWebView returns the underlying view, or nil before Embed has completed or
// after Close.
func (e *Chromium) GetWebView() *ICoreWebView2 {
	return e.webview
}

// SetVisible shows or hides the content area without destroying it.
func (e *Chromium) SetVisible(visible bool) error {
	if e.controller == nil {
		return nil
	}
	return e.controller.PutIsVisible(visible)
}

// Close tears the content view down and releases it.
//
// ShuttingDown first, so anything still in flight stops calling back into a
// half-released object; Wails does the same before destroying a window.
func (e *Chromium) Close() {
	e.ShuttingDown()
	if e.controller != nil {
		_ = e.controller.Close()
		e.controller = nil
	}
	e.webview = nil
}

// Exported constructors for the navigation handlers. The upstream ones are
// unexported because Chromium is their only consumer; a host driving its own
// toolbar subscribes to the same events.
//
// The parameter types are unexported interfaces, which callers outside this
// package cannot name but can satisfy, so passing a value still works.

func NewICoreWebView2NavigationStartingEventHandler(impl _ICoreWebView2NavigationStartingEventHandlerImpl) *ICoreWebView2NavigationStartingEventHandler {
	return newICoreWebView2NavigationStartingEventHandler(impl)
}

func NewICoreWebView2NavigationCompletedEventHandler(impl _ICoreWebView2NavigationCompletedEventHandlerImpl) *ICoreWebView2NavigationCompletedEventHandler {
	return newICoreWebView2NavigationCompletedEventHandler(impl)
}
