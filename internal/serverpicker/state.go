package serverpicker

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
)

const (
	stateVersion = 1
	stateName    = "ServerPicker.json"
	maxStateSize = 64 << 10
)

var ErrInvalidState = errors.New("invalid server picker state")

// State records which groups the user chose to block. It is intent, not truth:
// the firewall itself is truth, and the two are reconciled on load. SDRRevision
// lets a load notice that Valve reshuffled relay IPs since the rules were
// written, so a blocked group can be rewritten with its current addresses.
type State struct {
	Version       int      `json:"version"`
	BlockedGroups []string `json:"blockedGroups"`
	SDRRevision   int64    `json:"sdrRevision,omitempty"`
}

func defaultState() State { return State{Version: stateVersion, BlockedGroups: []string{}} }

func statePath() (string, error) {
	root, err := paths.DataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "Settings", stateName), nil
}

func loadState() (State, error) {
	path, err := statePath()
	if err != nil {
		return State{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultState(), nil
		}
		return State{}, err
	}
	defer f.Close()

	raw, err := io.ReadAll(io.LimitReader(f, maxStateSize+1))
	if err != nil {
		return State{}, err
	}
	if len(raw) > maxStateSize {
		return State{}, ErrInvalidState
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var st State
	if err := dec.Decode(&st); err != nil {
		return State{}, errors.Join(ErrInvalidState, err)
	}
	if dec.Decode(&struct{}{}) != io.EOF || st.Version != stateVersion {
		return State{}, ErrInvalidState
	}
	st.BlockedGroups = normalizeGroupIDs(st.BlockedGroups)
	return st, nil
}

func saveState(st State) error {
	st.Version = stateVersion
	st.BlockedGroups = normalizeGroupIDs(st.BlockedGroups)
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	path, err := statePath()
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, append(raw, '\n'), 0o600)
}

// normalizeGroupIDs trims, drops blanks, de-duplicates and sorts, so the file
// stays diff-stable no matter what order the UI toggled things in.
func normalizeGroupIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
