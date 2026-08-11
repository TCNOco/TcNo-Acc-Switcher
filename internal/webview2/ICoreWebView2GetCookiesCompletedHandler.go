//go:build windows

package webview2

type _ICoreWebView2GetCookiesCompletedHandlerVtbl struct {
	_IUnknownVtbl
	Invoke ComProc
}

// ICoreWebView2GetCookiesCompletedHandler receives the result of
// ICoreWebView2CookieManager.GetCookies.
type ICoreWebView2GetCookiesCompletedHandler struct {
	vtbl *_ICoreWebView2GetCookiesCompletedHandlerVtbl
	impl _ICoreWebView2GetCookiesCompletedHandlerImpl
}

type _ICoreWebView2GetCookiesCompletedHandlerImpl interface {
	_IUnknownImpl
	// GetCookiesCompleted receives the matched cookies. The list is owned by the
	// runtime for the duration of the call, so anything kept must be copied out.
	GetCookiesCompleted(errorCode uintptr, cookies *ICoreWebView2CookieList) uintptr
}

func _ICoreWebView2GetCookiesCompletedHandlerIUnknownQueryInterface(this *ICoreWebView2GetCookiesCompletedHandler, refiid, object uintptr) uintptr {
	return this.impl.QueryInterface(refiid, object)
}

func _ICoreWebView2GetCookiesCompletedHandlerIUnknownAddRef(this *ICoreWebView2GetCookiesCompletedHandler) uintptr {
	return this.impl.AddRef()
}

func _ICoreWebView2GetCookiesCompletedHandlerIUnknownRelease(this *ICoreWebView2GetCookiesCompletedHandler) uintptr {
	return this.impl.Release()
}

func _ICoreWebView2GetCookiesCompletedHandlerInvoke(this *ICoreWebView2GetCookiesCompletedHandler, errorCode uintptr, cookies *ICoreWebView2CookieList) uintptr {
	return this.impl.GetCookiesCompleted(errorCode, cookies)
}

var _ICoreWebView2GetCookiesCompletedHandlerFn = _ICoreWebView2GetCookiesCompletedHandlerVtbl{
	_IUnknownVtbl{
		NewComProc(_ICoreWebView2GetCookiesCompletedHandlerIUnknownQueryInterface),
		NewComProc(_ICoreWebView2GetCookiesCompletedHandlerIUnknownAddRef),
		NewComProc(_ICoreWebView2GetCookiesCompletedHandlerIUnknownRelease),
	},
	NewComProc(_ICoreWebView2GetCookiesCompletedHandlerInvoke),
}

func NewICoreWebView2GetCookiesCompletedHandler(impl _ICoreWebView2GetCookiesCompletedHandlerImpl) *ICoreWebView2GetCookiesCompletedHandler {
	return &ICoreWebView2GetCookiesCompletedHandler{
		vtbl: &_ICoreWebView2GetCookiesCompletedHandlerFn,
		impl: impl,
	}
}
