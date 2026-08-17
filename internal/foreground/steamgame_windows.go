//go:build windows

package foreground

import (
	"sync"

	"TcNo-Acc-Switcher/internal/crashlog"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Steam publishes the app id of whatever it is currently running under its own
// key, and clears it back to zero on exit. That is a far better "is a game up"
// signal than guessing from process names, and it catches windowed games, which
// fullscreen detection by definition cannot.
//
// RegNotifyChangeKeyValue means no polling: the goroutine parks on an event and
// the kernel wakes it when the key changes.
const (
	steamKeyPath      = `Software\Valve\Steam`
	runningAppIDValue = "RunningAppID"

	regNotifyChangeLastSet  = 0x00000004
	regNotifyThreadAgnostic = 0x10000000
)

var procRegNotifyChangeKeyValue = windows.NewLazySystemDLL("advapi32.dll").NewProc("RegNotifyChangeKeyValue")

var steamGame struct {
	mu   sync.Mutex
	stop chan struct{}
}

// WatchSteamGame reports whether Steam currently has a game running. onChange
// fires with the state at the time of the call, then on every transition.
func WatchSteamGame(onChange func(running bool)) func() {
	if onChange == nil {
		return func() {}
	}

	steamGame.mu.Lock()
	if steamGame.stop != nil {
		steamGame.mu.Unlock()
		return func() {}
	}
	stop := make(chan struct{})
	steamGame.stop = stop
	steamGame.mu.Unlock()

	onChange(steamGameRunning())

	go func() {
		defer crashlog.Capture()
		watchSteamGameKey(stop, onChange)
	}()

	return func() {
		steamGame.mu.Lock()
		cur := steamGame.stop
		steamGame.stop = nil
		steamGame.mu.Unlock()
		if cur != nil {
			close(cur)
		}
	}
}

func watchSteamGameKey(stop <-chan struct{}, onChange func(bool)) {
	key, err := registry.OpenKey(registry.CURRENT_USER, steamKeyPath, registry.NOTIFY|registry.QUERY_VALUE)
	if err != nil {
		// Steam not installed for this user; nothing will ever change.
		return
	}
	defer key.Close()

	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return
	}
	defer windows.CloseHandle(event)

	last := steamGameRunning()
	for {
		windows.ResetEvent(event)
		// Asynchronous: returns immediately and signals the event on change.
		// Thread-agnostic so the notification survives this goroutine being
		// rescheduled onto another OS thread.
		r, _, _ := procRegNotifyChangeKeyValue.Call(
			uintptr(key), 0,
			regNotifyChangeLastSet|regNotifyThreadAgnostic,
			uintptr(event), 1,
		)
		if r != 0 {
			return
		}

		waitStop := make(chan struct{})
		go func() {
			select {
			case <-stop:
				windows.SetEvent(event)
			case <-waitStop:
			}
		}()

		_, _ = windows.WaitForSingleObject(event, windows.INFINITE)
		close(waitStop)

		select {
		case <-stop:
			return
		default:
		}

		if now := steamGameRunning(); now != last {
			last = now
			onChange(now)
		}
	}
}

// steamGameRunning reads RunningAppID; anything non-zero is a game.
func steamGameRunning() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, steamKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()

	id, _, err := key.GetIntegerValue(runningAppIDValue)
	if err != nil {
		return false
	}
	return id != 0
}
