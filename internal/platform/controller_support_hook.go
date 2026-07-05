package platform

import "sync"

var (
	controllerSupportMu   sync.RWMutex
	controllerSupportHook func(bool)
)

func SetControllerSupportChangedHook(fn func(bool)) {
	controllerSupportMu.Lock()
	controllerSupportHook = fn
	controllerSupportMu.Unlock()
}

func TriggerControllerSupportChanged(enabled bool) {
	controllerSupportMu.RLock()
	fn := controllerSupportHook
	controllerSupportMu.RUnlock()
	if fn != nil {
		fn(enabled)
	}
}
