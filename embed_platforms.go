package main

import (
	"runtime"

	"TcNo-Acc-Switcher/internal/platform"

	_ "embed"
)

// All three catalogs ride in every binary. They are a few tens of kilobytes each,
// and carrying them means the catalog tests can check every platform's descriptors
// from whichever OS happens to be running them.
//
//go:embed Platforms.json
var windowsPlatformsJSON []byte

//go:embed Platforms.linux.json
var linuxPlatformsJSON []byte

//go:embed Platforms.mac.json
var macPlatformsJSON []byte

func init() {
	platform.SetEmbeddedPlatformsJSON(embeddedPlatformsForOS(runtime.GOOS))
}

// embeddedPlatformsForOS picks the catalog matching platform.PlatformsFileName.
// The two have to agree: the name decides which file on disk is read and updated,
// this decides what seeds it.
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
