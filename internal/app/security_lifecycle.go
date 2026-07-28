package app

import (
	"log/slog"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type securityLifecycleTrigger string

const (
	securityTriggerSystemSleep securityLifecycleTrigger = "system-sleep"
	securityTriggerScreenLock  securityLifecycleTrigger = "screen-lock"
	securityTriggerShutdown    securityLifecycleTrigger = "shutdown"
	securityTriggerSessionEnd  securityLifecycleTrigger = "session-end"
)

// Wails maps sleep across desktop platforms. ScreenLocked is currently emitted
// on mobile; Windows desktop coverage is added by security_lifecycle_windows.go.
// Wails exposes no desktop lock event for macOS or Linux in the pinned version.
var portableSecurityTriggers = [...]securityLifecycleTrigger{
	securityTriggerSystemSleep,
	securityTriggerScreenLock,
	securityTriggerShutdown,
}

type securityLifecycle struct {
	lock         func() error
	registerOnce sync.Once
	platformOnce sync.Once
}

func newSecurityLifecycle(lock func() error) *securityLifecycle {
	return &securityLifecycle{lock: lock}
}

func (l *securityLifecycle) handle(trigger securityLifecycleTrigger) {
	if l == nil || l.lock == nil {
		return
	}
	if err := l.lock(); err != nil {
		slog.Warn("security lifecycle lock failed", "trigger", trigger, "error", err)
	}
}

func (l *securityLifecycle) register(register func(securityLifecycleTrigger, func())) {
	if l == nil || register == nil {
		return
	}
	l.registerOnce.Do(func() {
		for _, trigger := range portableSecurityTriggers {
			trigger := trigger
			register(trigger, func() { l.handle(trigger) })
		}
	})
}

func registerSecurityLifecycle(app *application.App, lifecycle *securityLifecycle) {
	if app == nil || lifecycle == nil {
		return
	}
	lifecycle.register(func(trigger securityLifecycleTrigger, handler func()) {
		switch trigger {
		case securityTriggerSystemSleep:
			app.Event.OnApplicationEvent(events.Common.SystemWillSleep, func(*application.ApplicationEvent) { handler() })
		case securityTriggerScreenLock:
			app.Event.OnApplicationEvent(events.Common.ScreenLocked, func(*application.ApplicationEvent) { handler() })
		case securityTriggerShutdown:
			app.OnShutdown(handler)
		}
	})
}
