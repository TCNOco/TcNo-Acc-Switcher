//go:build android

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func init() {
	// In c-shared build mode, main() is not called automatically.
	application.RegisterAndroidMain(main)
}
