package main

import (
	"runtime"

	"TcNo-Acc-Switcher/internal/platform"

	_ "embed"
)

//go:embed Platforms.json
var windowsPlatformsJSON []byte

//go:embed Platforms.linux.json
var linuxPlatformsJSON []byte

//go:embed Platforms.mac.json
var macPlatformsJSON []byte

func init() {
	platform.SetEmbeddedPlatformsJSON(embeddedPlatformsForOS(runtime.GOOS))
}

// embeddedPlatformsForOS must agree with platform.PlatformsFileName: that names
// the file on disk, this decides what seeds it.
func embeddedPlatformsForOS(goos string) []byte {
	switch goos {
	case "windows":
		return windowsPlatformsJSON
	case "darwin":
		return macPlatformsJSON
	default:
		return linuxPlatformsJSON
	}
}
