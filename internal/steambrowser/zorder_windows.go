//go:build windows

package steambrowser

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The content view and the host window's own webview are sibling child windows
// of the same parent. Being created second should put the content view on top,
// but nothing guarantees it stays there: the host re-lays its webview out on
// resize and DPI change, and a sibling that ends up underneath is invisible
// while still reporting a loaded page and a working URL. That failure looks
// exactly like a page that will not load, so the view's position in the
// z-order is asserted rather than assumed.

var (
	user32             = windows.NewLazySystemDLL("user32.dll")
	procGetWindow      = user32.NewProc("GetWindow")
	procSetWindowPos   = user32.NewProc("SetWindowPos")
	procGetClassNameW  = user32.NewProc("GetClassNameW")
	procIsWindowVisble = user32.NewProc("IsWindowVisible")
	procIsZoomed       = user32.NewProc("IsZoomed")
	procGetSystemMet   = user32.NewProc("GetSystemMetrics")
)

const (
	smCXSizeFrame    = 32
	smCXPaddedBorder = 92

	gwHwndNext = 2
	gwChild    = 5

	hwndTop = 0

	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoActivate = 0x0010
)

// childWindows lists a window's direct children, in z-order, front to back.
func childWindows(parent uintptr) []uintptr {
	if parent == 0 {
		return nil
	}
	child, _, _ := procGetWindow.Call(parent, gwChild)
	if child == 0 {
		return nil
	}
	// GW_CHILD gives the topmost child; GW_HWNDNEXT walks down the z-order.
	children := []uintptr{child}
	for len(children) < 64 {
		next, _, _ := procGetWindow.Call(child, gwHwndNext)
		if next == 0 {
			break
		}
		children = append(children, next)
		child = next
	}
	return children
}

// newChildWindow returns the child present in after but not before, which is the
// one the call between the two snapshots created.
func newChildWindow(before, after []uintptr) uintptr {
	existing := make(map[uintptr]struct{}, len(before))
	for _, hwnd := range before {
		existing[hwnd] = struct{}{}
	}
	for _, hwnd := range after {
		if _, ok := existing[hwnd]; !ok {
			return hwnd
		}
	}
	return 0
}

// raiseWindow puts hwnd at the front of its siblings, without moving, resizing
// or focusing it.
func raiseWindow(hwnd uintptr) error {
	if hwnd == 0 {
		return fmt.Errorf("steambrowser: no content window to raise")
	}
	ret, _, err := procSetWindowPos.Call(
		hwnd, hwndTop,
		0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoActivate,
	)
	if ret == 0 {
		return fmt.Errorf("steambrowser: raise content view: %w", err)
	}
	return nil
}

// windowClassName is for logging only, so a window that ends up in front can be
// named rather than just numbered.
func windowClassName(hwnd uintptr) string {
	if hwnd == 0 {
		return ""
	}
	buffer := make([]uint16, 256)
	n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if n == 0 {
		return ""
	}
	return windows.UTF16ToString(buffer[:n])
}

func windowVisible(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	ret, _, _ := procIsWindowVisble.Call(hwnd)
	return ret != 0
}

// windowMaximised reports whether the host window is maximised, which is when
// its edges stop being resize handles.
func windowMaximised(hwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	ret, _, _ := procIsZoomed.Call(hwnd)
	return ret != 0
}

// resizeBorder is the width in physical pixels of the strip along a window's
// edges that Windows treats as a resize handle.
//
// A frameless window has no non-client area, so that strip lives inside the
// client area, and a child window covering the client area to its edges takes
// the mouse before the host can hit-test it. Leaving this much of the host
// exposed is what keeps the window resizable.
func resizeBorder() int {
	frame, _, _ := procGetSystemMet.Call(smCXSizeFrame)
	padded, _, _ := procGetSystemMet.Call(smCXPaddedBorder)
	border := int(frame) + int(padded)
	if border < 4 {
		// Some themes report no frame at all. A few pixels is still grabbable,
		// and is far better than an edge that cannot be hit.
		return 4
	}
	return border
}
