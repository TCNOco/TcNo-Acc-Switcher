//go:build windows

package webview2

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Event handlers the upstream binding did not declare. SourceChanged, HistoryChanged
// and DocumentTitleChanged are what keep a URL bar, its back/forward buttons and the
// window caption in step with the page; NewWindowRequested is what turns a
// middle-click into a window we control instead of a chromeless popup.
//
// Each follows the binding's existing shape: a vtable of IUnknown plus Invoke, a Go
// impl interface, and a constructor. The handler object must stay reachable from Go
// for as long as it is subscribed.

// --- SourceChanged ---

type _ICoreWebView2SourceChangedEventHandlerVtbl struct {
	_IUnknownVtbl
	Invoke ComProc
}

type ICoreWebView2SourceChangedEventHandler struct {
	vtbl *_ICoreWebView2SourceChangedEventHandlerVtbl
	impl _ICoreWebView2SourceChangedEventHandlerImpl
}

type _ICoreWebView2SourceChangedEventHandlerImpl interface {
	_IUnknownImpl
	SourceChanged(sender *ICoreWebView2, args uintptr) uintptr
}

func _sourceChangedQueryInterface(this *ICoreWebView2SourceChangedEventHandler, refiid, object uintptr) uintptr {
	return this.impl.QueryInterface(refiid, object)
}
func _sourceChangedAddRef(this *ICoreWebView2SourceChangedEventHandler) uintptr {
	return this.impl.AddRef()
}
func _sourceChangedRelease(this *ICoreWebView2SourceChangedEventHandler) uintptr {
	return this.impl.Release()
}
func _sourceChangedInvoke(this *ICoreWebView2SourceChangedEventHandler, sender *ICoreWebView2, args uintptr) uintptr {
	return this.impl.SourceChanged(sender, args)
}

var _sourceChangedFn = _ICoreWebView2SourceChangedEventHandlerVtbl{
	_IUnknownVtbl{
		NewComProc(_sourceChangedQueryInterface),
		NewComProc(_sourceChangedAddRef),
		NewComProc(_sourceChangedRelease),
	},
	NewComProc(_sourceChangedInvoke),
}

func NewICoreWebView2SourceChangedEventHandler(impl _ICoreWebView2SourceChangedEventHandlerImpl) *ICoreWebView2SourceChangedEventHandler {
	return &ICoreWebView2SourceChangedEventHandler{vtbl: &_sourceChangedFn, impl: impl}
}

// --- HistoryChanged ---

type _ICoreWebView2HistoryChangedEventHandlerVtbl struct {
	_IUnknownVtbl
	Invoke ComProc
}

type ICoreWebView2HistoryChangedEventHandler struct {
	vtbl *_ICoreWebView2HistoryChangedEventHandlerVtbl
	impl _ICoreWebView2HistoryChangedEventHandlerImpl
}

type _ICoreWebView2HistoryChangedEventHandlerImpl interface {
	_IUnknownImpl
	HistoryChanged(sender *ICoreWebView2, args uintptr) uintptr
}

func _historyChangedQueryInterface(this *ICoreWebView2HistoryChangedEventHandler, refiid, object uintptr) uintptr {
	return this.impl.QueryInterface(refiid, object)
}
func _historyChangedAddRef(this *ICoreWebView2HistoryChangedEventHandler) uintptr {
	return this.impl.AddRef()
}
func _historyChangedRelease(this *ICoreWebView2HistoryChangedEventHandler) uintptr {
	return this.impl.Release()
}
func _historyChangedInvoke(this *ICoreWebView2HistoryChangedEventHandler, sender *ICoreWebView2, args uintptr) uintptr {
	return this.impl.HistoryChanged(sender, args)
}

var _historyChangedFn = _ICoreWebView2HistoryChangedEventHandlerVtbl{
	_IUnknownVtbl{
		NewComProc(_historyChangedQueryInterface),
		NewComProc(_historyChangedAddRef),
		NewComProc(_historyChangedRelease),
	},
	NewComProc(_historyChangedInvoke),
}

func NewICoreWebView2HistoryChangedEventHandler(impl _ICoreWebView2HistoryChangedEventHandlerImpl) *ICoreWebView2HistoryChangedEventHandler {
	return &ICoreWebView2HistoryChangedEventHandler{vtbl: &_historyChangedFn, impl: impl}
}

// --- DocumentTitleChanged ---

type _ICoreWebView2DocumentTitleChangedEventHandlerVtbl struct {
	_IUnknownVtbl
	Invoke ComProc
}

type ICoreWebView2DocumentTitleChangedEventHandler struct {
	vtbl *_ICoreWebView2DocumentTitleChangedEventHandlerVtbl
	impl _ICoreWebView2DocumentTitleChangedEventHandlerImpl
}

type _ICoreWebView2DocumentTitleChangedEventHandlerImpl interface {
	_IUnknownImpl
	DocumentTitleChanged(sender *ICoreWebView2, args uintptr) uintptr
}

func _titleChangedQueryInterface(this *ICoreWebView2DocumentTitleChangedEventHandler, refiid, object uintptr) uintptr {
	return this.impl.QueryInterface(refiid, object)
}
func _titleChangedAddRef(this *ICoreWebView2DocumentTitleChangedEventHandler) uintptr {
	return this.impl.AddRef()
}
func _titleChangedRelease(this *ICoreWebView2DocumentTitleChangedEventHandler) uintptr {
	return this.impl.Release()
}
func _titleChangedInvoke(this *ICoreWebView2DocumentTitleChangedEventHandler, sender *ICoreWebView2, args uintptr) uintptr {
	return this.impl.DocumentTitleChanged(sender, args)
}

var _titleChangedFn = _ICoreWebView2DocumentTitleChangedEventHandlerVtbl{
	_IUnknownVtbl{
		NewComProc(_titleChangedQueryInterface),
		NewComProc(_titleChangedAddRef),
		NewComProc(_titleChangedRelease),
	},
	NewComProc(_titleChangedInvoke),
}

func NewICoreWebView2DocumentTitleChangedEventHandler(impl _ICoreWebView2DocumentTitleChangedEventHandlerImpl) *ICoreWebView2DocumentTitleChangedEventHandler {
	return &ICoreWebView2DocumentTitleChangedEventHandler{vtbl: &_titleChangedFn, impl: impl}
}

// --- NewWindowRequested ---

type iCoreWebView2NewWindowRequestedEventArgsVtbl struct {
	_IUnknownVtbl
	GetUri             ComProc
	PutNewWindow       ComProc
	GetNewWindow       ComProc
	PutHandled         ComProc
	GetHandled         ComProc
	GetIsUserInitiated ComProc
	GetDeferral        ComProc
	GetWindowFeatures  ComProc
}

// ICoreWebView2NewWindowRequestedEventArgs describes a popup or middle-click the
// page asked for. Setting Handled true stops WebView2 opening its own window,
// leaving the host free to route the URI wherever it belongs.
type ICoreWebView2NewWindowRequestedEventArgs struct {
	vtbl *iCoreWebView2NewWindowRequestedEventArgsVtbl
}

func (i *ICoreWebView2NewWindowRequestedEventArgs) GetUri() (string, error) {
	var uri *uint16
	hr, _, _ := i.vtbl.GetUri.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(&uri)),
	)
	if hr != 0 {
		return "", windows.Errno(hr)
	}
	return windows.UTF16PtrToString(uri), nil
}

func (i *ICoreWebView2NewWindowRequestedEventArgs) PutHandled(handled bool) error {
	var value int32
	if handled {
		value = 1
	}
	hr, _, _ := i.vtbl.PutHandled.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(value),
	)
	if hr != 0 {
		return windows.Errno(hr)
	}
	return nil
}

// GetIsUserInitiated distinguishes a middle-click or ctrl-click from a popup the
// page opened by itself.
func (i *ICoreWebView2NewWindowRequestedEventArgs) GetIsUserInitiated() (bool, error) {
	return boolProperty(i.vtbl.GetIsUserInitiated, unsafe.Pointer(i))
}

type _ICoreWebView2NewWindowRequestedEventHandlerVtbl struct {
	_IUnknownVtbl
	Invoke ComProc
}

type ICoreWebView2NewWindowRequestedEventHandler struct {
	vtbl *_ICoreWebView2NewWindowRequestedEventHandlerVtbl
	impl _ICoreWebView2NewWindowRequestedEventHandlerImpl
}

type _ICoreWebView2NewWindowRequestedEventHandlerImpl interface {
	_IUnknownImpl
	NewWindowRequested(sender *ICoreWebView2, args *ICoreWebView2NewWindowRequestedEventArgs) uintptr
}

func _newWindowQueryInterface(this *ICoreWebView2NewWindowRequestedEventHandler, refiid, object uintptr) uintptr {
	return this.impl.QueryInterface(refiid, object)
}
func _newWindowAddRef(this *ICoreWebView2NewWindowRequestedEventHandler) uintptr {
	return this.impl.AddRef()
}
func _newWindowRelease(this *ICoreWebView2NewWindowRequestedEventHandler) uintptr {
	return this.impl.Release()
}
func _newWindowInvoke(this *ICoreWebView2NewWindowRequestedEventHandler, sender *ICoreWebView2, args *ICoreWebView2NewWindowRequestedEventArgs) uintptr {
	return this.impl.NewWindowRequested(sender, args)
}

var _newWindowFn = _ICoreWebView2NewWindowRequestedEventHandlerVtbl{
	_IUnknownVtbl{
		NewComProc(_newWindowQueryInterface),
		NewComProc(_newWindowAddRef),
		NewComProc(_newWindowRelease),
	},
	NewComProc(_newWindowInvoke),
}

func NewICoreWebView2NewWindowRequestedEventHandler(impl _ICoreWebView2NewWindowRequestedEventHandlerImpl) *ICoreWebView2NewWindowRequestedEventHandler {
	return &ICoreWebView2NewWindowRequestedEventHandler{vtbl: &_newWindowFn, impl: impl}
}

// --- NavigationStarting args ---
//
// The binding passes these as a bare IUnknown, so the interface is declared here
// to reach Uri and Cancel. Slot order is from the IDL, where the type derives
// straight from IUnknown.

type iCoreWebView2NavigationStartingEventArgsVtbl struct {
	_IUnknownVtbl
	GetUri             ComProc
	GetIsUserInitiated ComProc
	GetIsRedirected    ComProc
	GetRequestHeaders  ComProc
	GetCancel          ComProc
	PutCancel          ComProc
	GetNavigationId    ComProc
}

// ICoreWebView2NavigationStartingEventArgs describes a navigation about to
// begin, and can refuse it.
type ICoreWebView2NavigationStartingEventArgs struct {
	vtbl *iCoreWebView2NavigationStartingEventArgsVtbl
}

// NavigationStartingArgs reinterprets the IUnknown a NavigationStarting handler
// receives. The runtime always passes this interface; the binding simply does
// not name it.
func NavigationStartingArgs(args *IUnknown) *ICoreWebView2NavigationStartingEventArgs {
	if args == nil {
		return nil
	}
	return (*ICoreWebView2NavigationStartingEventArgs)(unsafe.Pointer(args))
}

func (i *ICoreWebView2NavigationStartingEventArgs) GetUri() (string, error) {
	var uri *uint16
	hr, _, _ := i.vtbl.GetUri.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(&uri)),
	)
	if hr != 0 {
		return "", windows.Errno(hr)
	}
	return windows.UTF16PtrToString(uri), nil
}

// PutCancel refuses the navigation. The page stays where it is.
func (i *ICoreWebView2NavigationStartingEventArgs) PutCancel(cancel bool) error {
	var value int32
	if cancel {
		value = 1
	}
	hr, _, _ := i.vtbl.PutCancel.Call(uintptr(unsafe.Pointer(i)), uintptr(value))
	if hr != 0 {
		return windows.Errno(hr)
	}
	return nil
}
