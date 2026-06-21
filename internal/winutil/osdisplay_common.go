package winutil

import "runtime"

func archDisplay() string {
	if runtime.GOARCH == "amd64" {
		return "x64"
	}
	return runtime.GOARCH
}
