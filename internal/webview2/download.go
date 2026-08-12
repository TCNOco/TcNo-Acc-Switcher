//go:build windows

package webview2

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Downloads, which the upstream binding does not declare at all.
//
// A host that says nothing about downloads gets WebView2's own: a transfer into
// the profile's download folder and a flyout drawn over the page. Reaching
// DownloadStarting is what lets the host decide instead.
//
// Slot order is from the IDL. ICoreWebView2_4 derives from ICoreWebView2_3, so
// its vtable embeds that one rather than IUnknown - the interface's own four
// methods sit after every inherited slot, and starting from IUnknown would land
// every call on an unrelated method.

type iCoreWebView2_4Vtbl struct {
	iCoreWebView2_3Vtbl
	AddFrameCreated        ComProc
	RemoveFrameCreated     ComProc
	AddDownloadStarting    ComProc
	RemoveDownloadStarting ComProc
}

// ICoreWebView2_4 adds the download and frame events. Present from runtime
// 1.0.902.49; QueryInterface4 fails on anything older.
type ICoreWebView2_4 struct {
	vtbl *iCoreWebView2_4Vtbl
}

func (i *ICoreWebView2) QueryInterface4() (*ICoreWebView2_4, error) {
	var result *ICoreWebView2_4
	iid := windows.GUID{
		Data1: 0x20d02d59,
		Data2: 0x6df2,
		Data3: 0x42dc,
		Data4: [8]byte{0xbd, 0x06, 0xf9, 0x8a, 0x69, 0x4b, 0x13, 0x02},
	}
	hr, _, _ := i.vtbl.QueryInterface.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(&iid)),
		uintptr(unsafe.Pointer(&result)))
	if hr != 0 {
		return nil, windows.Errno(hr)
	}
	return result, nil
}

func (i *ICoreWebView2_4) Release() uint32 {
	ret, _, _ := i.vtbl.Release.Call(uintptr(unsafe.Pointer(i)))
	return uint32(ret)
}

func (i *ICoreWebView2_4) AddDownloadStarting(handler *ICoreWebView2DownloadStartingEventHandler) error {
	return addEventHandler(i.vtbl.AddDownloadStarting, unsafe.Pointer(i), unsafe.Pointer(handler))
}

// --- DownloadStarting args ---

type iCoreWebView2DownloadStartingEventArgsVtbl struct {
	_IUnknownVtbl
	GetDownloadOperation ComProc
	GetCancel            ComProc
	PutCancel            ComProc
	GetResultFilePath    ComProc
	PutResultFilePath    ComProc
	GetHandled           ComProc
	PutHandled           ComProc
	GetDeferral          ComProc
}

// ICoreWebView2DownloadStartingEventArgs describes a download about to begin,
// and can refuse it.
type ICoreWebView2DownloadStartingEventArgs struct {
	vtbl *iCoreWebView2DownloadStartingEventArgsVtbl
}

// GetDownloadOperation returns the operation being started. The caller owns a
// reference to it and must Release it.
func (i *ICoreWebView2DownloadStartingEventArgs) GetDownloadOperation() (*ICoreWebView2DownloadOperation, error) {
	var operation *ICoreWebView2DownloadOperation
	hr, _, _ := i.vtbl.GetDownloadOperation.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(&operation)),
	)
	if hr != 0 {
		return nil, windows.Errno(hr)
	}
	return operation, nil
}

// PutCancel stops the transfer before it begins.
func (i *ICoreWebView2DownloadStartingEventArgs) PutCancel(cancel bool) error {
	return putBool(i.vtbl.PutCancel, unsafe.Pointer(i), cancel)
}

// PutHandled suppresses the default download flyout. Cancelling alone still
// shows it, which would announce a download that is not happening.
func (i *ICoreWebView2DownloadStartingEventArgs) PutHandled(handled bool) error {
	return putBool(i.vtbl.PutHandled, unsafe.Pointer(i), handled)
}

// putBool writes a BOOL in-parameter, which COM takes as a 4-byte int.
func putBool(proc ComProc, this unsafe.Pointer, value bool) error {
	var raw int32
	if value {
		raw = 1
	}
	hr, _, _ := proc.Call(uintptr(this), uintptr(raw))
	if hr != 0 {
		return windows.Errno(hr)
	}
	return nil
}

// --- DownloadOperation ---

type iCoreWebView2DownloadOperationVtbl struct {
	_IUnknownVtbl
	AddBytesReceivedChanged       ComProc
	RemoveBytesReceivedChanged    ComProc
	AddEstimatedEndTimeChanged    ComProc
	RemoveEstimatedEndTimeChanged ComProc
	AddStateChanged               ComProc
	RemoveStateChanged            ComProc
	GetUri                        ComProc
	GetContentDisposition         ComProc
	GetMimeType                   ComProc
	GetTotalBytesToReceive        ComProc
	GetBytesReceived              ComProc
	GetEstimatedEndTime           ComProc
	GetResultFilePath             ComProc
	GetState                      ComProc
	GetInterruptReason            ComProc
	Cancel                        ComProc
	Pause                         ComProc
	Resume                        ComProc
	GetCanResume                  ComProc
}

// ICoreWebView2DownloadOperation is one download. Only its URI is read here;
// the rest of the interface describes progress this host never shows.
type ICoreWebView2DownloadOperation struct {
	vtbl *iCoreWebView2DownloadOperationVtbl
}

func (i *ICoreWebView2DownloadOperation) GetUri() (string, error) {
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

func (i *ICoreWebView2DownloadOperation) Release() uint32 {
	ret, _, _ := i.vtbl.Release.Call(uintptr(unsafe.Pointer(i)))
	return uint32(ret)
}

// --- DownloadStarting handler ---

type _ICoreWebView2DownloadStartingEventHandlerVtbl struct {
	_IUnknownVtbl
	Invoke ComProc
}

type ICoreWebView2DownloadStartingEventHandler struct {
	vtbl *_ICoreWebView2DownloadStartingEventHandlerVtbl
	impl _ICoreWebView2DownloadStartingEventHandlerImpl
}

type _ICoreWebView2DownloadStartingEventHandlerImpl interface {
	_IUnknownImpl
	DownloadStarting(sender *ICoreWebView2, args *ICoreWebView2DownloadStartingEventArgs) uintptr
}

func _downloadStartingQueryInterface(this *ICoreWebView2DownloadStartingEventHandler, refiid, object uintptr) uintptr {
	return this.impl.QueryInterface(refiid, object)
}
func _downloadStartingAddRef(this *ICoreWebView2DownloadStartingEventHandler) uintptr {
	return this.impl.AddRef()
}
func _downloadStartingRelease(this *ICoreWebView2DownloadStartingEventHandler) uintptr {
	return this.impl.Release()
}
func _downloadStartingInvoke(this *ICoreWebView2DownloadStartingEventHandler, sender *ICoreWebView2, args *ICoreWebView2DownloadStartingEventArgs) uintptr {
	return this.impl.DownloadStarting(sender, args)
}

var _downloadStartingFn = _ICoreWebView2DownloadStartingEventHandlerVtbl{
	_IUnknownVtbl{
		NewComProc(_downloadStartingQueryInterface),
		NewComProc(_downloadStartingAddRef),
		NewComProc(_downloadStartingRelease),
	},
	NewComProc(_downloadStartingInvoke),
}

func NewICoreWebView2DownloadStartingEventHandler(impl _ICoreWebView2DownloadStartingEventHandlerImpl) *ICoreWebView2DownloadStartingEventHandler {
	return &ICoreWebView2DownloadStartingEventHandler{vtbl: &_downloadStartingFn, impl: impl}
}
