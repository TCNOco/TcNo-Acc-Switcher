package stability

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"

	"github.com/google/uuid"
)

const countersFile = "StabilityCounters.json"

type countersState struct {
	Uuid      string         `json:"Uuid,omitempty"`
	Platforms map[string]int `json:"Platforms"`
}

var (
	mu     sync.Mutex
	loaded bool
	state  countersState
)

func countersPath() (string, error) {
	root, err := paths.DataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, countersFile), nil
}

func readStatsUUID() string {
	root, err := paths.DataRoot()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(root, "Statistics.json"))
	if err != nil {
		return ""
	}
	var aux struct {
		Uuid string `json:"Uuid"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return ""
	}
	return strings.TrimSpace(aux.Uuid)
}

func ensureLoadedLocked() error {
	if loaded {
		return nil
	}
	path, err := countersPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			state = countersState{Platforms: map[string]int{}}
			if u := readStatsUUID(); u != "" {
				state.Uuid = u
			} else {
				state.Uuid = uuid.NewString()
			}
			loaded = true
			return saveLocked()
		}
		return err
	}
	next := countersState{Platforms: map[string]int{}}
	if err := json.Unmarshal(data, &next); err != nil {
		state = countersState{Platforms: map[string]int{}}
		if u := readStatsUUID(); u != "" {
			state.Uuid = u
		} else {
			state.Uuid = uuid.NewString()
		}
		loaded = true
		return saveLocked()
	}
	if next.Platforms == nil {
		next.Platforms = map[string]int{}
	}
	if strings.TrimSpace(next.Uuid) == "" {
		if u := readStatsUUID(); u != "" {
			next.Uuid = u
		} else {
			next.Uuid = uuid.NewString()
		}
	}
	state = next
	loaded = true
	return nil
}

func saveLocked() error {
	path, err := countersPath()
	if err != nil {
		return err
	}
	out, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, out, 0o644)
}

// ClientUUID returns the anonymous install UUID used for API submissions.
func ClientUUID() (string, error) {
	mu.Lock()
	defer mu.Unlock()
	if err := ensureLoadedLocked(); err != nil {
		return "", err
	}
	return state.Uuid, nil
}

// incrementSwitchLocked increments the per-platform switch counter and returns the new count.
// Caller must hold mu.
func incrementSwitchLocked(platformKey string) (int, error) {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return 0, nil
	}
	if err := ensureLoadedLocked(); err != nil {
		return 0, err
	}
	state.Platforms[platformKey]++
	count := state.Platforms[platformKey]
	if err := saveLocked(); err != nil {
		return count, err
	}
	return count, nil
}

// ShouldPromptForCount reports whether a stability prompt should appear for this switch count.
func ShouldPromptForCount(count int) bool {
	return count == 1 || (count > 0 && count%10 == 0)
}
