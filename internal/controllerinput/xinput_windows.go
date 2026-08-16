//go:build windows

package controllerinput

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type xinputGamepad struct {
	Buttons      uint16
	LeftTrigger  byte
	RightTrigger byte
	ThumbLX      int16
	ThumbLY      int16
	ThumbRX      int16
	ThumbRY      int16
}

type xinputState struct {
	PacketNumber uint32
	Gamepad      xinputGamepad
}

type xinputReader struct {
	proc    *windows.LazyProc
	backoff slotBackoff
}

func newStateReader() stateReader {
	proc := resolveXInputProc()
	if proc == nil {
		controllerLog().Info("controller input unavailable: XInput not found")
		return nil
	}
	return &xinputReader{proc: proc}
}

func (r *xinputReader) Snapshots() []snapshot {
	out := make([]snapshot, 0, maxControllers)
	for i := 0; i < maxControllers; i++ {
		if !r.backoff.due(i) {
			out = append(out, snapshot{})
			continue
		}
		var state xinputState
		code := r.getState(uint32(i), &state)
		r.backoff.record(i, code == 0)
		if code != 0 {
			// XInput documents only ERROR_SUCCESS and ERROR_DEVICE_NOT_CONNECTED
			// here, and anything else it could fail with would not clear inside
			// one 16ms tick either, so every failure backs the slot off.
			out = append(out, snapshot{})
			continue
		}
		out = append(out, snapshot{
			Connected: true,
			Buttons:   state.Gamepad.Buttons,
			ThumbLX:   state.Gamepad.ThumbLX,
			ThumbLY:   state.Gamepad.ThumbLY,
		})
	}
	return out
}

func (r *xinputReader) getState(index uint32, state *xinputState) uint32 {
	ret, _, _ := r.proc.Call(uintptr(index), uintptr(unsafe.Pointer(state)))
	return uint32(ret)
}

func resolveXInputProc() *windows.LazyProc {
	for _, dllName := range []string{"xinput1_4.dll", "xinput1_3.dll", "xinput9_1_0.dll"} {
		dll := windows.NewLazySystemDLL(dllName)
		proc := dll.NewProc("XInputGetState")
		if err := dll.Load(); err != nil {
			continue
		}
		if err := proc.Find(); err != nil {
			continue
		}
		return proc
	}
	return nil
}
