//go:build !windows

package app

import "github.com/wailsapp/wails/v3/pkg/application"

func configurePlatformSecurityLifecycle(_ *application.Options, lifecycle *securityLifecycle) {
	if lifecycle != nil {
		lifecycle.platformOnce.Do(func() {})
	}
}
