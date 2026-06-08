package platform

import "TcNo-Acc-Switcher/internal/updatertheme"

func (*PlatformService) SetUpdaterThemeCSS(css string) {
	updatertheme.SetCSS(css)
}
