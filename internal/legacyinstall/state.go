package legacyinstall

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
)

const stateFileName = "LegacyCleanup.json"

// state records what the user has already decided about the leftover C# files.
// Only the refusal is worth keeping: a successful pass leaves nothing to find,
// so the scan alone decides everything else.
type state struct {
	Dismissed bool `json:"dismissed,omitempty"`
}

func statePath() (string, error) {
	root, err := paths.DataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, stateFileName), nil
}

// loadState reads the saved decision. A missing or unreadable file means none.
func loadState() state {
	path, err := statePath()
	if err != nil {
		return state{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			removeLog.Debug("read legacy cleanup state", "path", path, "err", err)
		}
		return state{}
	}
	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return state{}
	}
	return s
}

// Dismiss records that the user does not want the leftovers removed.
func Dismiss() error {
	path, err := statePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(state{Dismissed: true}, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, data, 0o644)
}

// ShouldPrompt reports whether the user should be asked to clean up dir: there
// are leftovers, removing them needs elevation, and they have not said no.
func ShouldPrompt(dir string) (Report, bool) {
	rep := Detect(dir)
	if !rep.Found() || Writable(dir) {
		return rep, false
	}
	return rep, !loadState().Dismissed
}
