// Package accountindex handles the on-disk account index shared by basic
// platforms and saved-account encryption.
package accountindex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"TcNo-Acc-Switcher/internal/fsutil"
)

var migrationMu sync.Mutex

// ReadFile upgrades the flat ID-to-name dictionary written by the .NET app to
// the v4 envelope. Account names and IDs stay unchanged, so their existing
// cache directories, shortcuts and order entries still refer to the same data.
// A byte-for-byte backup must be durable before the index can be replaced.
func ReadFile(path string) ([]byte, error) {
	migrationMu.Lock()
	defer migrationMu.Unlock()

	original, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data := bytes.TrimPrefix(original, []byte{0xef, 0xbb, 0xbf})
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		return nil, fmt.Errorf("account index must be a JSON object")
	}
	// An empty dictionary needs no conversion. Modern envelopes contain an
	// object (or null) under ids; a legacy account can itself be named "ids".
	if len(object) == 0 {
		return data, nil
	}
	ids := make(map[string]string, len(object))
	for key, value := range object {
		var name string
		if len(value) == 0 || value[0] != '"' || json.Unmarshal(value, &name) != nil {
			if value, ok := object["ids"]; ok {
				var current map[string]string
				if err := json.Unmarshal(value, &current); err == nil {
					return data, nil
				}
			}
			return nil, fmt.Errorf("unrecognized account index format")
		}
		ids[key] = name
	}
	converted, err := json.MarshalIndent(struct {
		IDs      map[string]string `json:"ids"`
		LastUsed map[string]string `json:"lastused"`
	}{ids, map[string]string{}}, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := backupLegacyIndex(path+".legacy-dotnet.bak", original); err != nil {
		return nil, fmt.Errorf("back up legacy account index: %w", err)
	}
	if err := fsutil.WriteFileAtomic(path, converted, 0o644); err != nil {
		return nil, fmt.Errorf("migrate legacy account index: %w", err)
	}
	return converted, nil
}

func backupLegacyIndex(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		previous, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if !bytes.Equal(previous, data) {
			return fmt.Errorf("an existing legacy backup differs from the account index")
		}
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	return f.Close()
}
