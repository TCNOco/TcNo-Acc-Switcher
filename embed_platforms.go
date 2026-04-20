package main

import (
	"TcNo-Acc-Switcher/internal/platform"

	_ "embed"
)

//go:embed Platforms.json
var embeddedPlatformsJSON []byte

func init() {
	platform.SetEmbeddedPlatformsJSON(embeddedPlatformsJSON)
}
