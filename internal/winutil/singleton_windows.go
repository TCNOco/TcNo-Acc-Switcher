//go:build windows

package winutil

import (
	"sync"

	"golang.org/x/sys/windows"
)

const singletonMutexName = "TcNo-Acc-Switcher-Singleton"

// TryAcquireSingleton creates the global mutex; returns release func if this process was first.
// If another instance holds the mutex, returns alreadyRunning=true and release=nil.
func TryAcquireSingleton() (release func(), alreadyRunning bool, err error) {
	name, err := windows.UTF16PtrFromString(singletonMutexName)
	if err != nil {
		return nil, false, err
	}
	h, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		if err == windows.ERROR_ALREADY_EXISTS {
			if h != 0 {
				_ = windows.CloseHandle(h)
			}
			return nil, true, nil
		}
		return nil, false, err
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			_ = windows.ReleaseMutex(h)
			_ = windows.CloseHandle(h)
		})
	}, false, nil
}
