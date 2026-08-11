//go:build windows

package webview2

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// The browsing surface a chrome-bearing window needs: where it is, where it can go
// back to, and when any of that changes.
//
// Everything hangs off ICoreWebView2_2 rather than ICoreWebView2 because that vtable
// models the full inherited chain, so one interface covers navigation, history,
// title, new-window requests and cookies without a second QueryInterface.

// GetSource returns the URI currently displayed. This is the URL-bar value; it
// updates on redirects and in-page history changes as well as on navigation.
func (i *ICoreWebView2_2) GetSource() (string, error) {
	var uri *uint16
	hr, _, _ := i.vtbl.GetSource.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(&uri)),
	)
	if hr != 0 {
		return "", windows.Errno(hr)
	}
	return windows.UTF16PtrToString(uri), nil
}

// GetDocumentTitle returns the current document's title, for the window caption.
func (i *ICoreWebView2_2) GetDocumentTitle() (string, error) {
	var title *uint16
	hr, _, _ := i.vtbl.GetDocumentTitle.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(&title)),
	)
	if hr != 0 {
		return "", windows.Errno(hr)
	}
	return windows.UTF16PtrToString(title), nil
}

func (i *ICoreWebView2_2) Navigate(uri string) error {
	uriPtr, err := windows.UTF16PtrFromString(uri)
	if err != nil {
		return err
	}
	hr, _, _ := i.vtbl.Navigate.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(uriPtr)),
	)
	if hr != 0 {
		return windows.Errno(hr)
	}
	return nil
}

func (i *ICoreWebView2_2) Reload() error {
	hr, _, _ := i.vtbl.Reload.Call(uintptr(unsafe.Pointer(i)))
	if hr != 0 {
		return windows.Errno(hr)
	}
	return nil
}

func (i *ICoreWebView2_2) Stop() error {
	hr, _, _ := i.vtbl.Stop.Call(uintptr(unsafe.Pointer(i)))
	if hr != 0 {
		return windows.Errno(hr)
	}
	return nil
}

func (i *ICoreWebView2_2) GoBack() error {
	hr, _, _ := i.vtbl.GoBack.Call(uintptr(unsafe.Pointer(i)))
	if hr != 0 {
		return windows.Errno(hr)
	}
	return nil
}

func (i *ICoreWebView2_2) GoForward() error {
	hr, _, _ := i.vtbl.GoForward.Call(uintptr(unsafe.Pointer(i)))
	if hr != 0 {
		return windows.Errno(hr)
	}
	return nil
}

// boolProperty reads a BOOL out-parameter, which COM returns as a 4-byte int.
func boolProperty(proc ComProc, this unsafe.Pointer) (bool, error) {
	var value int32
	hr, _, _ := proc.Call(uintptr(this), uintptr(unsafe.Pointer(&value)))
	if hr != 0 {
		return false, windows.Errno(hr)
	}
	return value != 0, nil
}

func (i *ICoreWebView2_2) GetCanGoBack() (bool, error) {
	return boolProperty(i.vtbl.GetCanGoBack, unsafe.Pointer(i))
}

func (i *ICoreWebView2_2) GetCanGoForward() (bool, error) {
	return boolProperty(i.vtbl.GetCanGoForward, unsafe.Pointer(i))
}

func (i *ICoreWebView2_2) AddSourceChanged(handler *ICoreWebView2SourceChangedEventHandler, token *_EventRegistrationToken) error {
	return addEventHandler(i.vtbl.AddSourceChanged, unsafe.Pointer(i), unsafe.Pointer(handler), token)
}

func (i *ICoreWebView2_2) AddHistoryChanged(handler *ICoreWebView2HistoryChangedEventHandler, token *_EventRegistrationToken) error {
	return addEventHandler(i.vtbl.AddHistoryChanged, unsafe.Pointer(i), unsafe.Pointer(handler), token)
}

func (i *ICoreWebView2_2) AddDocumentTitleChanged(handler *ICoreWebView2DocumentTitleChangedEventHandler, token *_EventRegistrationToken) error {
	return addEventHandler(i.vtbl.AddDocumentTitleChanged, unsafe.Pointer(i), unsafe.Pointer(handler), token)
}

func (i *ICoreWebView2_2) AddNewWindowRequested(handler *ICoreWebView2NewWindowRequestedEventHandler, token *_EventRegistrationToken) error {
	return addEventHandler(i.vtbl.AddNewWindowRequested, unsafe.Pointer(i), unsafe.Pointer(handler), token)
}

func (i *ICoreWebView2_2) AddNavigationStarting(handler *ICoreWebView2NavigationStartingEventHandler, token *_EventRegistrationToken) error {
	return addEventHandler(i.vtbl.AddNavigationStarting, unsafe.Pointer(i), unsafe.Pointer(handler), token)
}

func (i *ICoreWebView2_2) AddNavigationCompleted(handler *ICoreWebView2NavigationCompletedEventHandler, token *_EventRegistrationToken) error {
	return addEventHandler(i.vtbl.AddNavigationCompleted, unsafe.Pointer(i), unsafe.Pointer(handler), token)
}

func addEventHandler(proc ComProc, this, handler unsafe.Pointer, token *_EventRegistrationToken) error {
	hr, _, _ := proc.Call(
		uintptr(this),
		uintptr(handler),
		uintptr(unsafe.Pointer(token)),
	)
	if hr != 0 {
		return windows.Errno(hr)
	}
	return nil
}
