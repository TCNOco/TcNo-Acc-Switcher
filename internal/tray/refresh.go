package tray

import "github.com/wailsapp/wails/v3/pkg/application"

var menuRefresh func()

func SetMenuRefresh(fn func()) {
	menuRefresh = fn
}

func RefreshMenuIfSet() {
	if menuRefresh == nil {
		return
	}
	application.InvokeSync(menuRefresh)
}
