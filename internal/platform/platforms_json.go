package platform

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"TcNo-Acc-Switcher/internal/updatecheck"
)

var (
	platformsJSONCache struct {
		mu     sync.RWMutex
		exeDir string
		data   []byte
		loaded bool
	}
)

func invalidatePlatformsJSONCache() {
	platformsJSONCache.mu.Lock()
	platformsJSONCache.loaded = false
	platformsJSONCache.data = nil
	platformsJSONCache.mu.Unlock()
}

// LoadPlatformsJSON returns the effective platforms configuration: the base
// Platforms.json (see [ResolvePlatformsJSONPath]) after optional merge with
// {UserDataDir}/Platforms.custom.json. Matching platform keys in the custom file
// replace the base entry; new keys are added. Custom file must be valid JSON with
// a top-level "Platforms" object.
//
// When the default base Platforms.json is missing under the user data folder,
// it is created from the embedded catalog (first run). An existing file is
// replaced only when the embedded catalog carries a newer Version, matching the
// rule the background update already uses; otherwise it is left alone. Keep
// local edits in Platforms.custom.json, which merges over the base and is never
// overwritten.
//
// The returned slice is cached internally and shared across callers; it must be
// treated as read-only. Callers that need to mutate the bytes should make a copy.
func LoadPlatformsJSON(exeDir string) ([]byte, error) {
	exeDir = filepath.Clean(exeDir)

	platformsJSONCache.mu.RLock()
	if platformsJSONCache.loaded && platformsJSONCache.exeDir == exeDir {
		data := platformsJSONCache.data
		platformsJSONCache.mu.RUnlock()
		return data, nil
	}
	platformsJSONCache.mu.RUnlock()

	if err := seedEmbeddedPlatforms(exeDir); err != nil {
		return nil, err
	}
	s, err := loadSettings(exeDir)
	if err != nil {
		return nil, err
	}
	basePath := resolvePlatformsPath(exeDir, s)
	base, err := os.ReadFile(basePath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", basePath, err)
	}
	if s.PlatformsJSONPath == "" {
		if merged, changed, err := addEmbeddedSteamPlatform(base, embeddedPlatformsJSON); err != nil {
			return nil, fmt.Errorf("restore embedded Steam platform: %w", err)
		} else if changed {
			if err := atomicWriteBytes(basePath, merged, 0o644); err != nil {
				return nil, fmt.Errorf("write %s: %w", basePath, err)
			}
			base = merged
		}
	}
	customPath := filepath.Join(UserDataDir(exeDir), "Platforms.custom.json")
	custom, err := os.ReadFile(customPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			platformsJSONCache.mu.Lock()
			platformsJSONCache.exeDir = exeDir
			platformsJSONCache.data = bytes.Clone(base)
			platformsJSONCache.loaded = true
			platformsJSONCache.mu.Unlock()
			return platformsJSONCache.data, nil
		}
		return nil, fmt.Errorf("read %s: %w", customPath, err)
	}
	out, err := mergePlatformsJSON(base, custom)
	if err != nil {
		return nil, fmt.Errorf("merge Platforms.custom.json: %w", err)
	}
	platformsJSONCache.mu.Lock()
	platformsJSONCache.exeDir = exeDir
	platformsJSONCache.data = bytes.Clone(out)
	platformsJSONCache.loaded = true
	platformsJSONCache.mu.Unlock()
	return platformsJSONCache.data, nil
}

func seedEmbeddedPlatforms(exeDir string) error {
	if len(embeddedPlatformsJSON) == 0 {
		return nil
	}
	ud := UserDataDir(exeDir)
	if err := os.MkdirAll(ud, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(ud, "Platforms.json")
	st, err := os.Stat(dest)
	switch {
	case err == nil && !st.IsDir():
		if !embeddedPlatformsSupersede(dest) {
			return nil
		}
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return err
	}
	return atomicWriteBytes(dest, bytes.Clone(embeddedPlatformsJSON), 0o644)
}

// embeddedPlatformsSupersede reports whether the embedded catalog is newer than
// the copy already at path.
//
// A local file that is unreadable or carries no Version counts as older on
// purpose: the pre-v4 C# releases wrote their catalog to this same location
// without a Version field, so an in-place upgrade would otherwise pin the user
// to a pre-rewrite catalog forever and no descriptor fix would ever reach them.
func embeddedPlatformsSupersede(path string) bool {
	embeddedVer, err := updatecheck.ParsePlatformsJSONVersion(embeddedPlatformsJSON)
	if err != nil {
		return false
	}
	local, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	localVer, _ := updatecheck.ParsePlatformsJSONVersion(local)
	return updatecheck.IsVersionNewer(embeddedVer, localVer)
}

func mergePlatformsJSON(base, overlay []byte) ([]byte, error) {
	var main, over platformsFile
	if err := json.Unmarshal(base, &main); err != nil {
		return nil, err
	}
	if main.Platforms == nil {
		main.Platforms = map[string]json.RawMessage{}
	}
	if err := json.Unmarshal(overlay, &over); err != nil {
		return nil, err
	}
	if over.Platforms == nil {
		return base, nil
	}
	for k, v := range over.Platforms {
		main.Platforms[k] = v
	}
	return json.Marshal(main)
}
