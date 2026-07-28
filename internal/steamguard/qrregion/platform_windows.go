//go:build windows

package qrregion

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"TcNo-Acc-Switcher/internal/i18n"
	"TcNo-Acc-Switcher/internal/platform"
)

const (
	csHRedraw = 0x0002
	csVRedraw = 0x0001

	wsPopup        = 0x80000000
	wsExTopmost    = 0x00000008
	wsExToolWindow = 0x00000080
	wsExLayered    = 0x00080000

	lwaAlpha = 0x00000002

	swHide = 0
	swShow = 5

	wmDestroy       = 0x0002
	wmActivate      = 0x0006
	wmSetFocus      = 0x0007
	wmKillFocus     = 0x0008
	wmPaint         = 0x000F
	wmEraseBkgnd    = 0x0014
	wmCancelMode    = 0x001F
	wmDisplayChange = 0x007E
	wmKeyDown       = 0x0100
	wmSysKeyDown    = 0x0104
	wmMouseMove     = 0x0200
	wmLButtonDown   = 0x0201
	wmLButtonUp     = 0x0202
	wmRButtonDown   = 0x0204
	wmContextCancel = 0x8001

	vkEscape   = 0x1B
	waInactive = 0

	monitorDefaultToNearest = 2

	colorWindow = 5

	psSolid = 0

	dpiAwarenessPerMonitorV2 = ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4)

	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79

	idcCross = 32515

	bkModeTransparent = 1

	dtCenter     = 0x00000001
	dtVCenter    = 0x00000004
	dtSingleLine = 0x00000020
	dtNoPrefix   = 0x00000800

	fontWeightSemibold   = 600
	fontCharsetDefault   = 1
	fontOutPrecisTT      = 4
	fontClipDefault      = 0
	fontQualityCleartype = 5
	fontPitchDefault     = 0

	overlayAlpha = 150 // whole-window layered alpha; the wash/selection contrast is painted, not alpha-blended

	instructionFontHeight = 22
	instructionBandTop    = 48
	instructionBandHeight = 56
	instructionBandWidth  = 900

	selectionBorderWidth = 2
)

var (
	user32Region                     = windows.NewLazySystemDLL("user32.dll")
	gdi32Region                      = windows.NewLazySystemDLL("gdi32.dll")
	dwmapiRegion                     = windows.NewLazySystemDLL("dwmapi.dll")
	kernel32Region                   = windows.NewLazySystemDLL("kernel32.dll")
	procRegisterClassExW             = user32Region.NewProc("RegisterClassExW")
	procCreateWindowExW              = user32Region.NewProc("CreateWindowExW")
	procDefWindowProcW               = user32Region.NewProc("DefWindowProcW")
	procDestroyWindow                = user32Region.NewProc("DestroyWindow")
	procShowWindow                   = user32Region.NewProc("ShowWindow")
	procUpdateWindow                 = user32Region.NewProc("UpdateWindow")
	procSetForegroundWindow          = user32Region.NewProc("SetForegroundWindow")
	procSetFocus                     = user32Region.NewProc("SetFocus")
	procSetCapture                   = user32Region.NewProc("SetCapture")
	procReleaseCapture               = user32Region.NewProc("ReleaseCapture")
	procInvalidateRect               = user32Region.NewProc("InvalidateRect")
	procBeginPaint                   = user32Region.NewProc("BeginPaint")
	procEndPaint                     = user32Region.NewProc("EndPaint")
	procFillRect                     = user32Region.NewProc("FillRect")
	procFrameRect                    = user32Region.NewProc("FrameRect")
	procGetClientRect                = user32Region.NewProc("GetClientRect")
	procGetCursorPos                 = user32Region.NewProc("GetCursorPos")
	procMonitorFromPoint             = user32Region.NewProc("MonitorFromPoint")
	procGetMonitorInfoW              = user32Region.NewProc("GetMonitorInfoW")
	procGetSystemMetrics             = user32Region.NewProc("GetSystemMetrics")
	procLoadCursorW                  = user32Region.NewProc("LoadCursorW")
	procDrawTextW                    = user32Region.NewProc("DrawTextW")
	procGetMessageW                  = user32Region.NewProc("GetMessageW")
	procTranslateMessage             = user32Region.NewProc("TranslateMessage")
	procDispatchMessageW             = user32Region.NewProc("DispatchMessageW")
	procPostMessageW                 = user32Region.NewProc("PostMessageW")
	procSetLayeredWindowAttributes   = user32Region.NewProc("SetLayeredWindowAttributes")
	procSetThreadDpiAwarenessContext = user32Region.NewProc("SetThreadDpiAwarenessContext")
	procSetViewportOrgEx             = gdi32Region.NewProc("SetViewportOrgEx")
	procCreateSolidBrush             = gdi32Region.NewProc("CreateSolidBrush")
	procDeleteObjectOverlay          = gdi32Region.NewProc("DeleteObject")
	procCreateFontW                  = gdi32Region.NewProc("CreateFontW")
	procSetTextColor                 = gdi32Region.NewProc("SetTextColor")
	procSetBkMode                    = gdi32Region.NewProc("SetBkMode")
	procDwmFlush                     = dwmapiRegion.NewProc("DwmFlush")
	procGetModuleHandleW             = kernel32Region.NewProc("GetModuleHandleW")

	overlayClassOnce sync.Once
	overlayClassErr  error
	overlayClassName *uint16
	overlayWndProc   = syscall.NewCallback(regionWindowProc)
	overlayStates    sync.Map
)

type windowsBackend struct{}

type nativePoint struct{ X, Y int32 }

type nativeRect struct{ Left, Top, Right, Bottom int32 }

type monitorInfo struct {
	Size    uint32
	Monitor nativeRect
	Work    nativeRect
	Flags   uint32
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSmall  uintptr
}

type nativeMessage struct {
	Window  uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Point   nativePoint
	Private uint32
}

type paintStruct struct {
	DC        uintptr
	Erase     int32
	Paint     nativeRect
	Restore   int32
	IncUpdate int32
	Reserved  [32]byte
}

type overlayState struct {
	// virtualScreen is the whole virtual desktop in physical screen pixels. Its
	// origin can be negative (a monitor placed left of / above the primary), so
	// every client<->screen conversion has to go through it.
	virtualScreen Rect
	// instructionBand is in client coordinates, centred on the monitor that held
	// the cursor when the overlay opened.
	instructionBand nativeRect
	instruction     []uint16
	start           nativePoint
	current         nativePoint
	dragging        bool
	completed       bool
	selection       Rect
	err             error
}

func newBackend() backend { return windowsBackend{} }

func (windowsBackend) selectRegion(ctx context.Context) (Rect, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	previousDPI, _, callErr := procSetThreadDpiAwarenessContext.Call(dpiAwarenessPerMonitorV2)
	if previousDPI == 0 {
		return Rect{}, platformError(callErr)
	}
	defer procSetThreadDpiAwarenessContext.Call(previousDPI)

	virtualScreen, err := virtualScreenBounds()
	if err != nil {
		return Rect{}, err
	}
	if err := registerOverlayClass(); err != nil {
		return Rect{}, err
	}
	state := &overlayState{virtualScreen: virtualScreen}
	// Best-effort: the overlay still works if we cannot locate the cursor monitor
	// or read the localized string, we just centre the text on the whole desktop.
	cursorMonitor, monitorErr := monitorAtCursor()
	if monitorErr != nil {
		cursorMonitor = virtualScreen
	}
	state.instructionBand = instructionBandRect(virtualScreen, cursorMonitor)
	if text, textErr := windows.UTF16FromString(overlayInstructionText()); textErr == nil {
		state.instruction = text
	}
	window, err := createOverlayWindow(virtualScreen)
	if err != nil {
		return Rect{}, err
	}
	overlayStates.Store(window, state)
	defer overlayStates.Delete(window)
	defer func() {
		if isWindow(window) {
			procDestroyWindow.Call(window)
		}
	}()

	stopContext := make(chan struct{})
	contextExited := make(chan struct{})
	defer func() {
		close(stopContext)
		<-contextExited
	}()
	go func() {
		defer close(contextExited)
		select {
		case <-ctx.Done():
			select {
			case <-stopContext:
				return
			default:
				procPostMessageW.Call(window, wmContextCancel, 0, 0)
			}
		case <-stopContext:
		}
	}()

	procShowWindow.Call(window, swShow)
	procUpdateWindow.Call(window)
	procSetForegroundWindow.Call(window)
	procSetFocus.Call(window)

	var message nativeMessage
	for !state.completed {
		result, _, msgErr := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) == -1 {
			return Rect{}, platformError(msgErr)
		}
		if result == 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
	if state.err != nil {
		return Rect{}, state.err
	}
	if !state.completed || !state.selection.Valid() {
		return Rect{}, canceled(CancelEmptyRegion)
	}
	return state.selection, nil
}

// virtualScreenBounds returns the union of every monitor in physical pixels.
// The thread is per-monitor-v2 DPI aware by this point, so GetSystemMetrics
// reports true physical pixels and no further scaling is needed anywhere.
func virtualScreenBounds() (Rect, error) {
	left := systemMetric(smXVirtualScreen)
	top := systemMetric(smYVirtualScreen)
	width := systemMetric(smCXVirtualScreen)
	height := systemMetric(smCYVirtualScreen)
	if width <= 0 || height <= 0 || width > 32767 || height > 32767 {
		return Rect{}, errors.Join(ErrCapture, errors.New("virtual screen bounds are invalid"))
	}
	return Rect{Left: left, Top: top, Right: left + width, Bottom: top + height}, nil
}

func systemMetric(index int32) int32 {
	value, _, _ := procGetSystemMetrics.Call(uintptr(uint32(index)))
	return int32(uint32(value))
}

// instructionBandRect centres the instruction band horizontally on the cursor's
// monitor, near its top edge, expressed in overlay client coordinates.
func instructionBandRect(virtualScreen, monitor Rect) nativeRect {
	width := min32(instructionBandWidth, monitor.Width())
	centerX := monitor.Left + monitor.Width()/2 - virtualScreen.Left
	left := centerX - width/2
	top := monitor.Top - virtualScreen.Top + instructionBandTop
	return nativeRect{Left: left, Top: top, Right: left + width, Bottom: top + instructionBandHeight}
}

func overlayInstructionText() string {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return i18n.T("", "en-US", "SteamGuard_QROverlay_Instruction", nil)
	}
	language := "en-US"
	if settings, err := platform.LoadAppSettings(exeDir); err == nil && strings.TrimSpace(settings.Language) != "" {
		language = settings.Language
	}
	return i18n.T(exeDir, language, "SteamGuard_QROverlay_Instruction", nil)
}

func monitorAtCursor() (Rect, error) {
	var point nativePoint
	result, _, callErr := procGetCursorPos.Call(uintptr(unsafe.Pointer(&point)))
	if result == 0 {
		return Rect{}, platformError(callErr)
	}
	packed := uintptr(uint32(point.X)) | uintptr(uint64(uint32(point.Y))<<32)
	monitor, _, callErr := procMonitorFromPoint.Call(packed, monitorDefaultToNearest)
	if monitor == 0 {
		return Rect{}, platformError(callErr)
	}
	info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
	result, _, callErr = procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info)))
	if result == 0 {
		return Rect{}, platformError(callErr)
	}
	region := Rect(info.Monitor)
	if !region.Valid() || region.Width() > 32767 || region.Height() > 32767 {
		return Rect{}, errors.Join(ErrCapture, errors.New("monitor bounds are invalid"))
	}
	return region, nil
}

func registerOverlayClass() error {
	overlayClassOnce.Do(func() {
		className, err := windows.UTF16PtrFromString("TcNoSteamQRRegionOverlay")
		if err != nil {
			overlayClassErr = platformError(err)
			return
		}
		overlayClassName = className
		instance, _, callErr := procGetModuleHandleW.Call(0)
		if instance == 0 {
			overlayClassErr = platformError(callErr)
			return
		}
		// A crosshair beats inheriting whatever cursor (often the busy spinner) the
		// calling window had.
		cursor, _, _ := procLoadCursorW.Call(0, idcCross)
		class := wndClassEx{
			Size: uint32(unsafe.Sizeof(wndClassEx{})), Style: csHRedraw | csVRedraw,
			WndProc: overlayWndProc, Instance: instance, Cursor: cursor,
			Background: colorWindow + 1, ClassName: className,
		}
		atom, _, callErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))
		if atom == 0 {
			overlayClassErr = platformError(callErr)
		}
	})
	return overlayClassErr
}

func createOverlayWindow(bounds Rect) (uintptr, error) {
	instance, _, callErr := procGetModuleHandleW.Call(0)
	if instance == 0 {
		return 0, platformError(callErr)
	}
	title, _ := windows.UTF16PtrFromString("Select Steam login QR code")
	window, _, callErr := procCreateWindowExW.Call(
		wsExTopmost|wsExToolWindow|wsExLayered,
		uintptr(unsafe.Pointer(overlayClassName)), uintptr(unsafe.Pointer(title)), wsPopup,
		uintptr(uint32(bounds.Left)), uintptr(uint32(bounds.Top)),
		uintptr(uint32(bounds.Width())), uintptr(uint32(bounds.Height())),
		0, 0, instance, 0,
	)
	if window == 0 {
		return 0, platformError(callErr)
	}
	result, _, callErr := procSetLayeredWindowAttributes.Call(window, 0, overlayAlpha, lwaAlpha)
	if result == 0 {
		procDestroyWindow.Call(window)
		return 0, platformError(callErr)
	}
	return window, nil
}

func regionWindowProc(window uintptr, message uint32, wParam, lParam uintptr) uintptr {
	loaded, exists := overlayStates.Load(window)
	if !exists {
		result, _, _ := procDefWindowProcW.Call(window, uintptr(message), wParam, lParam)
		return result
	}
	state := loaded.(*overlayState)
	switch message {
	case wmEraseBkgnd:
		return 1
	case wmPaint:
		paintOverlay(window, state)
		return 0
	case wmLButtonDown:
		state.dragging = true
		state.start = clientPoint(lParam, state.virtualScreen)
		state.current = state.start
		procSetCapture.Call(window)
		procInvalidateRect.Call(window, 0, 0)
		return 0
	case wmMouseMove:
		if state.dragging {
			previous := clientSelectionRect(state)
			state.current = clientPoint(lParam, state.virtualScreen)
			// Only what the rectangle just covered or now covers. Invalidating the
			// whole window repainted the entire virtual desktop for every mouse
			// message, which is the other half of why a drag flashed.
			damage := dragDamageRect(previous, clientSelectionRect(state))
			procInvalidateRect.Call(window, uintptr(unsafe.Pointer(&damage)), 0)
		}
		return 0
	case wmLButtonUp:
		if state.dragging {
			state.current = clientPoint(lParam, state.virtualScreen)
			state.dragging = false
			procReleaseCapture.Call()
			selection := selectionRect(state)
			if !selection.Valid() {
				finishOverlay(window, state, Rect{}, canceled(CancelEmptyRegion))
			} else {
				finishOverlay(window, state, selection, nil)
			}
		}
		return 0
	case wmRButtonDown:
		finishOverlay(window, state, Rect{}, canceled(CancelRightClick))
		return 0
	case wmKeyDown, wmSysKeyDown:
		if wParam == vkEscape {
			finishOverlay(window, state, Rect{}, canceled(CancelEscape))
			return 0
		}
	case wmActivate:
		if uint16(wParam) == waInactive && !state.completed {
			finishOverlay(window, state, Rect{}, canceled(CancelFocusLost))
		}
		return 0
	case wmKillFocus, wmCancelMode:
		if !state.completed {
			finishOverlay(window, state, Rect{}, canceled(CancelFocusLost))
		}
		return 0
	case wmDisplayChange:
		finishOverlay(window, state, Rect{}, canceled(CancelDisplayChange))
		return 0
	case wmContextCancel:
		finishOverlay(window, state, Rect{}, canceled(CancelContext))
		return 0
	case wmDestroy:
		if !state.completed {
			state.completed = true
			state.err = canceled(CancelFocusLost)
		}
		return 0
	case wmSetFocus:
		return 0
	}
	result, _, _ := procDefWindowProcW.Call(window, uintptr(message), wParam, lParam)
	return result
}

func finishOverlay(window uintptr, state *overlayState, selection Rect, err error) {
	if state.completed {
		return
	}
	state.completed = true
	state.selection = selection
	state.err = err
	procReleaseCapture.Call()
	procShowWindow.Call(window, swHide)
	if result, _, callErr := procDwmFlush.Call(); result != 0 && state.err == nil {
		state.selection = Rect{}
		state.err = platformError(callErr)
	}
	procDestroyWindow.Call(window)
}

func paintOverlay(window uintptr, state *overlayState) {
	var paint paintStruct
	dc, _, _ := procBeginPaint.Call(window, uintptr(unsafe.Pointer(&paint)))
	if dc == 0 {
		return
	}
	defer procEndPaint.Call(window, uintptr(unsafe.Pointer(&paint)))
	var client nativeRect
	if result, _, _ := procGetClientRect.Call(window, uintptr(unsafe.Pointer(&client))); result == 0 {
		return
	}
	// The frame is assembled off-screen and blitted in one piece. Painting straight
	// onto the window let the compositor show the wash before the selection landed
	// on top of it, which is what made a drag flash.
	target, present := backBuffer(dc, paint.Paint)
	defer present()
	paintScene(target, state, client)
}

// backBuffer hands back a device context to draw one frame into, and a function
// that puts it on screen and releases it. The bitmap covers only the damaged
// rectangle; the origin is shifted so the painter can keep working in client
// coordinates. If any of it fails the window's own DC is drawn on instead — a
// flickering overlay still selects a region, a missing one does not.
func backBuffer(dc uintptr, damage nativeRect) (uintptr, func()) {
	width, height := damage.Right-damage.Left, damage.Bottom-damage.Top
	if width <= 0 || height <= 0 {
		return dc, func() {}
	}
	memory, _, _ := procCreateCompatibleDC.Call(dc)
	if memory == 0 {
		return dc, func() {}
	}
	// Compatible with the window DC, not the memory DC: a fresh memory DC holds a
	// 1x1 monochrome bitmap, so measuring against it would paint in black and white.
	bitmap, _, _ := procCreateCompatibleBitmap.Call(dc, uintptr(uint32(width)), uintptr(uint32(height)))
	if bitmap == 0 {
		procDeleteDC.Call(memory)
		return dc, func() {}
	}
	previous, _, _ := procSelectObject.Call(memory, bitmap)
	procSetViewportOrgEx.Call(memory, uintptr(uint32(-damage.Left)), uintptr(uint32(-damage.Top)), 0)
	return memory, func() {
		procBitBlt.Call(
			dc, uintptr(uint32(damage.Left)), uintptr(uint32(damage.Top)),
			uintptr(uint32(width)), uintptr(uint32(height)),
			memory, uintptr(uint32(damage.Left)), uintptr(uint32(damage.Top)), srcCopy,
		)
		if previous != 0 {
			procSelectObject.Call(memory, previous)
		}
		procDeleteObjectOverlay.Call(bitmap)
		procDeleteDC.Call(memory)
	}
}

// paintScene draws the whole overlay every time. It is clipped to the damaged
// area by the device context, so a partial repaint needs no special case here.
func paintScene(dc uintptr, state *overlayState, client nativeRect) {
	// The layered alpha is uniform across the window, so a real "hole" is not
	// possible. Instead the whole overlay gets a near-white wash (the desktop
	// reads as faded) and the selection interior gets a near-black fill, so the
	// selected area shows through markedly darker and starker than its surround.
	wash, _, _ := procCreateSolidBrush.Call(rgb(240, 240, 245))
	if wash != 0 {
		procFillRect.Call(dc, uintptr(unsafe.Pointer(&client)), wash)
	}
	if state.dragging {
		if selection := clientSelectionRect(state); selection.Right > selection.Left && selection.Bottom > selection.Top {
			if interior, _, _ := procCreateSolidBrush.Call(rgb(20, 24, 32)); interior != 0 {
				procFillRect.Call(dc, uintptr(unsafe.Pointer(&selection)), interior)
				procDeleteObjectOverlay.Call(interior)
			}
			if border, _, _ := procCreateSolidBrush.Call(rgb(102, 192, 244)); border != 0 {
				for offset := int32(0); offset < selectionBorderWidth; offset++ {
					frame := inflateRect(selection, offset)
					procFrameRect.Call(dc, uintptr(unsafe.Pointer(&frame)), border)
				}
				procDeleteObjectOverlay.Call(border)
			}
		}
	}
	paintInstruction(dc, state, wash)
	if wash != 0 {
		procDeleteObjectOverlay.Call(wash)
	}
}

// paintInstruction draws the localized hint over a wash-filled band so it stays
// legible even when the selection rectangle runs underneath it.
func paintInstruction(dc uintptr, state *overlayState, wash uintptr) {
	if len(state.instruction) <= 1 {
		return
	}
	band := state.instructionBand
	if band.Right <= band.Left || band.Bottom <= band.Top {
		return
	}
	if wash != 0 {
		procFillRect.Call(dc, uintptr(unsafe.Pointer(&band)), wash)
	}
	faceName, err := windows.UTF16PtrFromString("Segoe UI")
	if err != nil {
		return
	}
	font, _, _ := procCreateFontW.Call(
		uintptr(^uintptr(instructionFontHeight-1)), 0, 0, 0, fontWeightSemibold, 0, 0, 0,
		fontCharsetDefault, fontOutPrecisTT, fontClipDefault, fontQualityCleartype, fontPitchDefault,
		uintptr(unsafe.Pointer(faceName)),
	)
	if font == 0 {
		return
	}
	previous, _, _ := procSelectObject.Call(dc, font)
	procSetBkMode.Call(dc, bkModeTransparent)
	format := uintptr(dtCenter | dtVCenter | dtSingleLine | dtNoPrefix)
	// A light halo offset down-right, then the dark glyphs on top: readable
	// against both the wash and any dark selection fill that overlaps the band.
	halo := nativeRect{Left: band.Left + 2, Top: band.Top + 2, Right: band.Right + 2, Bottom: band.Bottom + 2}
	procSetTextColor.Call(dc, rgb(255, 255, 255))
	procDrawTextW.Call(dc, uintptr(unsafe.Pointer(&state.instruction[0])), ^uintptr(0), uintptr(unsafe.Pointer(&halo)), format)
	procSetTextColor.Call(dc, rgb(18, 22, 30))
	procDrawTextW.Call(dc, uintptr(unsafe.Pointer(&state.instruction[0])), ^uintptr(0), uintptr(unsafe.Pointer(&band)), format)
	runtime.KeepAlive(state.instruction)
	if previous != 0 {
		procSelectObject.Call(dc, previous)
	}
	procDeleteObjectOverlay.Call(font)
}

// dragDamageRect is everything the selection rectangle touched between two mouse
// positions: both rectangles, widened to take in the border drawn outside each.
func dragDamageRect(previous, current nativeRect) nativeRect {
	union := nativeRect{
		Left: min32(previous.Left, current.Left), Top: min32(previous.Top, current.Top),
		Right: max32(previous.Right, current.Right), Bottom: max32(previous.Bottom, current.Bottom),
	}
	return inflateRect(union, selectionBorderWidth+1)
}

func inflateRect(rect nativeRect, amount int32) nativeRect {
	return nativeRect{
		Left: rect.Left - amount, Top: rect.Top - amount,
		Right: rect.Right + amount, Bottom: rect.Bottom + amount,
	}
}

func clientPoint(lParam uintptr, monitor Rect) nativePoint {
	x := int32(int16(uint16(lParam)))
	y := int32(int16(uint16(lParam >> 16)))
	return nativePoint{X: clamp32(x, 0, monitor.Width()), Y: clamp32(y, 0, monitor.Height())}
}

func clientSelectionRect(state *overlayState) nativeRect {
	return nativeRect{
		Left: min32(state.start.X, state.current.X), Top: min32(state.start.Y, state.current.Y),
		Right: max32(state.start.X, state.current.X), Bottom: max32(state.start.Y, state.current.Y),
	}
}

func selectionRect(state *overlayState) Rect {
	client := clientSelectionRect(state)
	return Rect{
		Left: state.virtualScreen.Left + client.Left, Top: state.virtualScreen.Top + client.Top,
		Right: state.virtualScreen.Left + client.Right, Bottom: state.virtualScreen.Top + client.Bottom,
	}
}

func isWindow(window uintptr) bool {
	proc := user32Region.NewProc("IsWindow")
	result, _, _ := proc.Call(window)
	return result != 0
}

func platformError(callErr error) error {
	if callErr == nil || errors.Is(callErr, syscall.Errno(0)) {
		return ErrCapture
	}
	return errors.Join(ErrCapture, callErr)
}

func rgb(red, green, blue byte) uintptr {
	return uintptr(uint32(red) | uint32(green)<<8 | uint32(blue)<<16)
}

func clamp32(value, low, high int32) int32 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
