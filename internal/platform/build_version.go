package platform

import (
	buildinfo "TcNo-Acc-Switcher/build"
)

func appVersionFromBuildConfig() string {
	return buildinfo.Version()
}
