package platform

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
)

// ExeLocationDefaultList unmarshals from JSON as either a single string or an array of strings.
// Entries are trimmed; empty strings are dropped. Resolution tries each path in order until one exists.
type ExeLocationDefaultList []string

func (e *ExeLocationDefaultList) UnmarshalJSON(data []byte) error {
	*e = nil
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		s = strings.TrimSpace(s)
		if s != "" {
			*e = ExeLocationDefaultList{s}
		}
		return nil
	}
	var arr []string
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	for _, s := range arr {
		s = strings.TrimSpace(s)
		if s != "" {
			*e = append(*e, s)
		}
	}
	return nil
}

// MarshalJSON encodes a single path as a string; multiple paths as a JSON array.
func (e ExeLocationDefaultList) MarshalJSON() ([]byte, error) {
	if len(e) == 0 {
		return []byte(`""`), nil
	}
	if len(e) == 1 {
		return json.Marshal(e[0])
	}
	return json.Marshal([]string(e))
}

// FirstExpanded returns the first path with a non-empty expanded form (does not check the filesystem).
func (e ExeLocationDefaultList) FirstExpanded() string {
	for _, raw := range e {
		exp := ExpandWindowsPath(strings.TrimSpace(raw))
		if exp != "" {
			return exp
		}
	}
	return ""
}

// FirstExistingExe returns the first expanded path that exists and is not a directory.
func (e ExeLocationDefaultList) FirstExistingExe() string {
	for _, raw := range e {
		exp := ExpandWindowsPath(strings.TrimSpace(raw))
		if exp == "" {
			continue
		}
		if st, err := os.Stat(exp); err == nil && !st.IsDir() {
			return exp
		}
	}
	return ""
}
