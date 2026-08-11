//go:build windows

package webview2

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

// Named profiles are how one process gives each account its own cookie jar. They
// live under a single user data folder and require ICoreWebView2Environment10,
// which the upstream binding did not cover, so the interfaces are declared here.
//
// Slot layout is taken from the WebView2 IDL, not inferred. ICoreWebView2Environment10
// derives from ICoreWebView2Environment9, so seventeen inherited slots sit between
// IUnknown and its own methods. The padding below reproduces that and is load
// bearing: get it wrong and the call lands on a different method, returns a
// plausible HRESULT, and fails much later somewhere unrelated.

// ErrProfilesUnsupported reports a WebView2 runtime older than 110, which cannot
// separate storage by profile.
var ErrProfilesUnsupported = errors.New("webview2: runtime does not support profiles (requires 110 or newer)")

type iCoreWebView2ControllerOptionsVtbl struct {
	_IUnknownVtbl
	GetProfileName            ComProc
	PutProfileName            ComProc
	GetIsInPrivateModeEnabled ComProc
	PutIsInPrivateModeEnabled ComProc
}

// ICoreWebView2ControllerOptions carries the profile a controller is created with.
type ICoreWebView2ControllerOptions struct {
	vtbl *iCoreWebView2ControllerOptionsVtbl
}

func (i *ICoreWebView2ControllerOptions) PutProfileName(name string) error {
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	hr, _, _ := i.vtbl.PutProfileName.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(namePtr)),
	)
	if hr != 0 {
		return syscall.Errno(hr)
	}
	return nil
}

func (i *ICoreWebView2ControllerOptions) Release() uintptr {
	refCounter, _, _ := i.vtbl.Release.Call(uintptr(unsafe.Pointer(i)))
	return refCounter
}

type iCoreWebView2Environment10Vtbl struct {
	_IUnknownVtbl
	// ICoreWebView2Environment (5), Environment2 (1), Environment3 (2),
	// Environment4 (1), Environment5 (2), Environment6 (1), Environment7 (1),
	// Environment8 (3), Environment9 (1).
	_                                                  [17]ComProc
	CreateCoreWebView2ControllerOptions                ComProc
	CreateCoreWebView2ControllerWithOptions            ComProc
	CreateCoreWebView2CompositionControllerWithOptions ComProc
}

type ICoreWebView2Environment10 struct {
	vtbl *iCoreWebView2Environment10Vtbl
}

// GetICoreWebView2Environment10 returns nil on runtimes without profile support.
func (i *ICoreWebView2Environment) GetICoreWebView2Environment10() *ICoreWebView2Environment10 {
	var result *ICoreWebView2Environment10
	iid := NewGUID("{ee0eb9df-6f12-46ce-b53f-3f47b9c928e0}")
	hr, _, _ := i.vtbl.QueryInterface.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&result)),
	)
	if hr != 0 {
		return nil
	}
	return result
}

func (i *ICoreWebView2Environment10) CreateCoreWebView2ControllerOptions() (*ICoreWebView2ControllerOptions, error) {
	var options *ICoreWebView2ControllerOptions
	hr, _, _ := i.vtbl.CreateCoreWebView2ControllerOptions.Call(
		uintptr(unsafe.Pointer(i)),
		uintptr(unsafe.Pointer(&options)),
	)
	if hr != 0 {
		return nil, syscall.Errno(hr)
	}
	return options, nil
}

func (i *ICoreWebView2Environment10) CreateCoreWebView2ControllerWithOptions(
	hwnd uintptr,
	options *ICoreWebView2ControllerOptions,
	handler *iCoreWebView2CreateCoreWebView2ControllerCompletedHandler,
) error {
	hr, _, _ := i.vtbl.CreateCoreWebView2ControllerWithOptions.Call(
		uintptr(unsafe.Pointer(i)),
		hwnd,
		uintptr(unsafe.Pointer(options)),
		uintptr(unsafe.Pointer(handler)),
	)
	if hr != 0 {
		return syscall.Errno(hr)
	}
	return nil
}

// SupportsProfiles reports whether this environment can separate storage by profile.
func (e *ICoreWebView2Environment) SupportsProfiles() bool {
	return e.GetICoreWebView2Environment10() != nil
}

// createControllerWithProfile starts a content view on hwnd whose cookies,
// storage and cache are isolated to the named profile.
//
// Completion is asynchronous, so the caller must be pumping messages, and the
// handler has to outlive this call. It is taken as an argument rather than
// built here for that reason: the caller holds it in a field the garbage
// collector can see, whereas one created here would be unreachable the moment
// this returns and freed under the runtime.
//
// The controller handed to the handler is owned by the runtime for the duration
// of that call and must be AddRef'd to outlive it.
func createControllerWithProfile(
	env *ICoreWebView2Environment,
	hwnd uintptr,
	profile string,
	handler *iCoreWebView2CreateCoreWebView2ControllerCompletedHandler,
) error {
	if env == nil {
		return errors.New("webview2: nil environment")
	}
	if err := ValidateProfileName(profile); err != nil {
		return err
	}
	env10 := env.GetICoreWebView2Environment10()
	if env10 == nil {
		return ErrProfilesUnsupported
	}
	options, err := env10.CreateCoreWebView2ControllerOptions()
	if err != nil {
		return fmt.Errorf("webview2: create controller options: %w", err)
	}
	defer options.Release()
	if err := options.PutProfileName(profile); err != nil {
		return fmt.Errorf("webview2: set profile %q: %w", profile, err)
	}
	if err := env10.CreateCoreWebView2ControllerWithOptions(hwnd, options, handler); err != nil {
		return fmt.Errorf("webview2: create controller for profile %q: %w", profile, err)
	}
	return nil
}
