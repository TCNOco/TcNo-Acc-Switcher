//go:build windows

package winutil

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows"
)

// Use Explorer's desktop automation object, rather than creating a Shell.Application
// in our elevated process. This also leaves the target outside our process tree/job.
// https://devblogs.microsoft.com/oldnewthing/20131118-00/?p=2643
func shellExecuteAsDesktopUser(exe string, args []string, workingDir string, hideWindow bool) error {
	resolved, err := resolveExecutablePath(exe)
	if err != nil {
		return err
	}
	if workingDir == "" && filepath.IsAbs(resolved) {
		workingDir = filepath.Dir(resolved)
	}
	// ShellExecute takes a parameter string, not argv. Use Windows argv quoting;
	// no command interpreter is involved, so metacharacters remain literal.
	return withDesktopShell(func(shell *ole.IDispatch) error {
		show := int32(windows.SW_SHOWNORMAL)
		if hideWindow {
			show = windows.SW_HIDE
		}
		result, err := oleutil.CallMethod(shell, "ShellExecute", resolved, shellExecuteParameters(args), workingDir, "open", show)
		if result != nil {
			defer result.Clear()
		}
		return err
	})
}

func shellExecuteParameters(args []string) string {
	parameters := make([]string, len(args))
	for i, arg := range args {
		parameters[i] = syscall.EscapeArg(arg)
	}
	return strings.Join(parameters, " ")
}

func withDesktopShell(use func(*ole.IDispatch) error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitialize(0); err != nil {
		if oe, ok := err.(*ole.OleError); !ok || oe.Code() != 1 { // S_FALSE still needs Uninitialize.
			return fmt.Errorf("desktop COM initialization: %w", err)
		}
	}
	defer ole.CoUninitialize()

	shell, err := desktopShellDispatch()
	if err != nil {
		return err
	}
	defer shell.Release()
	return use(shell)
}

func desktopShellDispatch() (*ole.IDispatch, error) {
	shellWindows, err := ole.CreateInstance(ole.NewGUID("{9BA05972-F6A8-11CF-A442-00A0C90A8F39}"), ole.IID_IDispatch)
	if err != nil {
		return nil, fmt.Errorf("create ShellWindows: %w", err)
	}
	defer shellWindows.Release()
	location := ole.NewVariant(ole.VT_I4, 0) // CSIDL_DESKTOP
	var unused ole.VARIANT
	var hwnd int32
	desktop, err := oleutil.CallMethod((*ole.IDispatch)(unsafe.Pointer(shellWindows)), "FindWindowSW", &location, &unused, int32(8), &hwnd, int32(1)) // SWC_DESKTOP, SWFO_NEEDDISPATCH
	if desktop != nil {
		defer desktop.Clear()
	}
	if err != nil {
		return nil, fmt.Errorf("find Explorer desktop: %w", err)
	}
	if desktop == nil || desktop.ToIDispatch() == nil {
		return nil, fmt.Errorf("Explorer desktop is unavailable")
	}
	provider, err := desktop.ToIDispatch().QueryInterface(ole.NewGUID("{6D5140C1-7436-11CE-8034-00AA006009FA}")) // IServiceProvider
	if err != nil {
		return nil, fmt.Errorf("desktop IServiceProvider: %w", err)
	}
	defer provider.Release()

	// IServiceProvider::QueryService is slot 3 after IUnknown.
	var browser *ole.IUnknown
	hr, _, _ := syscall.SyscallN((*[4]uintptr)(unsafe.Pointer(provider.RawVTable))[3],
		uintptr(unsafe.Pointer(provider)),
		uintptr(unsafe.Pointer(ole.NewGUID("{4C96BE40-915C-11CF-99D3-00AA004AE837}"))), // SID_STopLevelBrowser
		uintptr(unsafe.Pointer(ole.NewGUID("{000214E2-0000-0000-C000-000000000046}"))), // IShellBrowser
		uintptr(unsafe.Pointer(&browser)))
	if err := shellInterfaceResult("desktop shell browser", hr, browser); err != nil {
		return nil, err
	}
	defer browser.Release()

	// IShellBrowser::QueryActiveShellView is slot 15 (including IUnknown/IOleWindow).
	var view *ole.IUnknown
	hr, _, _ = syscall.SyscallN((*[18]uintptr)(unsafe.Pointer(browser.RawVTable))[15], uintptr(unsafe.Pointer(browser)), uintptr(unsafe.Pointer(&view)))
	if err := shellInterfaceResult("desktop shell view", hr, view); err != nil {
		return nil, err
	}
	defer view.Release()

	// IShellView::GetItemObject is slot 15; SVGIO_BACKGROUND obtains ShellFolderView.
	var folder *ole.IDispatch
	hr, _, _ = syscall.SyscallN((*[16]uintptr)(unsafe.Pointer(view.RawVTable))[15], uintptr(unsafe.Pointer(view)),
		0, uintptr(unsafe.Pointer(ole.IID_IDispatch)), uintptr(unsafe.Pointer(&folder)))
	if err := shellInterfaceResult("desktop folder automation", hr, (*ole.IUnknown)(unsafe.Pointer(folder))); err != nil {
		return nil, err
	}
	defer folder.Release()
	application, err := oleutil.GetProperty(folder, "Application")
	if application != nil {
		defer application.Clear()
	}
	if err != nil {
		return nil, fmt.Errorf("desktop shell application: %w", err)
	}
	if application == nil || application.ToIDispatch() == nil {
		return nil, fmt.Errorf("desktop shell application is unavailable")
	}
	shell := application.ToIDispatch()
	shell.AddRef() // The caller owns a reference after application.Clear.
	return shell, nil
}

func shellInterfaceResult(operation string, hr uintptr, object *ole.IUnknown) error {
	if int32(hr) < 0 {
		return fmt.Errorf("%s: %w", operation, ole.NewError(hr))
	}
	if object == nil {
		return fmt.Errorf("%s is unavailable", operation)
	}
	return nil
}
