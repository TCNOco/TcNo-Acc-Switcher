package platform

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"TcNo-Acc-Switcher/internal/updatecheck"
	"TcNo-Acc-Switcher/internal/crashlog"
)

const (
	PlatformsJSONUpdateFoundEvent = "platforms-json-update-found"
	PlatformsJSONUpdatedEvent     = "platforms-json-updated"

	platformsUpdateTimeout = 30 * time.Second
)

type PlatformsJSONUpdatePayload struct {
	Version string `json:"version"`
}

func emitPlatformsJSONUpdateFound(version string) {
	app := application.Get()
	if app == nil {
		return
	}
	_ = app.Event.Emit(PlatformsJSONUpdateFoundEvent, PlatformsJSONUpdatePayload{Version: version})
}

func emitPlatformsJSONUpdated(version string) {
	app := application.Get()
	if app == nil {
		return
	}
	_ = app.Event.Emit(PlatformsJSONUpdatedEvent, PlatformsJSONUpdatePayload{Version: version})
}

func runLaunchPlatformsJSONCheck(exeDir string) {
	defer crashlog.Capture()
	ctx, cancel := context.WithTimeout(context.Background(), platformsUpdateTimeout)
	defer cancel()

	s, err := loadSettings(exeDir)
	if err != nil || strings.TrimSpace(s.PlatformsJSONPath) != "" {
		return
	}

	destPath := filepath.Join(UserDataDir(exeDir), "Platforms.json")
	localRaw, err := os.ReadFile(destPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if len(embeddedPlatformsJSON) == 0 {
				return
			}
			localRaw = embeddedPlatformsJSON
		} else {
			return
		}
	}

	localVer, _ := updatecheck.ParsePlatformsJSONVersion(localRaw)

	remoteRaw, err := updatecheck.FetchRemotePlatformsJSON(ctx, appVersionFromBuildConfig())
	if err != nil {
		return
	}

	remoteVer, err := updatecheck.ParsePlatformsJSONVersion(remoteRaw)
	if err != nil {
		return
	}
	if !updatecheck.IsVersionNewer(remoteVer, localVer) {
		return
	}
	if _, err := parsePlatformNames(remoteRaw); err != nil {
		return
	}

	emitPlatformsJSONUpdateFound(remoteVer)

	if err := os.MkdirAll(UserDataDir(exeDir), 0o755); err != nil {
		return
	}
	if err := atomicWriteBytes(destPath, remoteRaw, 0o644); err != nil {
		return
	}
	invalidatePlatformsJSONCache()
	emitPlatformsJSONUpdated(remoteVer)
}
