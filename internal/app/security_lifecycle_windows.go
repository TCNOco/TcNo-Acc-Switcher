//go:build windows

package app

import (
	"log/slog"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

const (
	wmDestroy          = 0x0002
	wmQueryEndSession  = 0x0011
	wmEndSession       = 0x0016
	wmWTSSessionChange = 0x02B1

	wtsSessionLogoff = 0x6
	wtsSessionLock   = 0x7
	wtsThisSession   = 0
)

var (
	wtsapi32                         = windows.NewLazySystemDLL("wtsapi32.dll")
	wtsRegisterSessionNotification   = wtsapi32.NewProc("WTSRegisterSessionNotification")
	wtsUnregisterSessionNotification = wtsapi32.NewProc("WTSUnRegisterSessionNotification")
)

type windowsSessionNotifications struct {
	mu         sync.Mutex
	registered map[uintptr]struct{}
	loggedFail bool
}

func configurePlatformSecurityLifecycle(options *application.Options, lifecycle *securityLifecycle) {
	if options == nil || lifecycle == nil {
		return
	}
	lifecycle.platformOnce.Do(func() {
		prior := options.Windows.WndProcInterceptor
		notifications := &windowsSessionNotifications{registered: make(map[uintptr]struct{})}
		options.Windows.WndProcInterceptor = func(hwnd uintptr, msg uint32, wParam, lParam uintptr) (uintptr, bool) {
			notifications.observe(hwnd, msg)
			if trigger, ok := windowsSecurityTrigger(msg, wParam); ok {
				lifecycle.handle(trigger)
			}
			if prior != nil {
				return prior(hwnd, msg, wParam, lParam)
			}
			return 0, false
		}
	})
}

// WTS supplies current-session workstation lock and logoff notifications. The
// end-session messages also cover a normal Windows shutdown before app teardown.
func windowsSecurityTrigger(msg uint32, wParam uintptr) (securityLifecycleTrigger, bool) {
	switch msg {
	case wmQueryEndSession:
		return securityTriggerSessionEnd, true
	case wmEndSession:
		if wParam != 0 {
			return securityTriggerSessionEnd, true
		}
	case wmWTSSessionChange:
		if wParam == wtsSessionLock || wParam == wtsSessionLogoff {
			return securityTriggerScreenLock, true
		}
	}
	return "", false
}

func (n *windowsSessionNotifications) observe(hwnd uintptr, msg uint32) {
	if hwnd == 0 {
		return
	}
	if msg == wmDestroy {
		n.unregister(hwnd)
		return
	}
	n.register(hwnd)
}

func (n *windowsSessionNotifications) register(hwnd uintptr) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, ok := n.registered[hwnd]; ok {
		return
	}
	result, _, err := wtsRegisterSessionNotification.Call(hwnd, wtsThisSession)
	if result == 0 {
		if !n.loggedFail {
			slog.Warn("Windows session notification registration failed", "error", err)
			n.loggedFail = true
		}
		return
	}
	n.registered[hwnd] = struct{}{}
}

func (n *windowsSessionNotifications) unregister(hwnd uintptr) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, ok := n.registered[hwnd]; !ok {
		return
	}
	if result, _, err := wtsUnregisterSessionNotification.Call(hwnd); result == 0 {
		slog.Warn("Windows session notification cleanup failed", "error", err)
	}
	delete(n.registered, hwnd)
}
