package platform

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// LoadPlatformsJSON returns the effective platforms configuration: the base
// Platforms.json (see [ResolvePlatformsJSONPath]) after optional merge with
// {UserDataDir}/Platforms.custom.json. Matching platform keys in the custom file
// replace the base entry; new keys are added. Custom file must be valid JSON with
// a top-level "Platforms" object.
//
// When the default base Platforms.json is missing under the user data folder,
// it is created from the embedded catalog (first run). An existing file is left
// unchanged so a copy applied via the UI persists until you use Restore default.
func LoadPlatformsJSON(exeDir string) ([]byte, error) {
	exeDir = filepath.Clean(exeDir)
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
	customPath := filepath.Join(UserDataDir(exeDir), "Platforms.custom.json")
	custom, err := os.ReadFile(customPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return base, nil
		}
		return nil, fmt.Errorf("read %s: %w", customPath, err)
	}
	out, err := mergePlatformsJSON(base, custom)
	if err != nil {
		return nil, fmt.Errorf("merge Platforms.custom.json: %w", err)
	}
	return out, nil
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
	if st, err := os.Stat(dest); err == nil && !st.IsDir() {
		return nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return atomicWriteBytes(dest, bytes.Clone(embeddedPlatformsJSON), 0o644)
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
