package platforms

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type geforceNowUserJSON struct {
	PreferredUsername string `json:"preferred_username"`
}

// GeForceNowSuggestedName reads Chromium cache blobs under dataPattern (glob),
// newest first, and returns preferred_username from the first parseable JSON object.
func GeForceNowSuggestedName(dataPattern string) (string, error) {
	files, err := filepath.Glob(strings.TrimSpace(dataPattern))
	if err != nil || len(files) == 0 {
		return "", err
	}
	sortByModDesc(files)
	for _, f := range files {
		st, err := os.Stat(f)
		if err != nil || st.IsDir() {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil || len(data) == 0 {
			continue
		}
		if n := parseGeForceNowPreferredUsername(data); n != "" {
			return n, nil
		}
	}
	return "", nil
}

func parseGeForceNowPreferredUsername(data []byte) string {
	data = bytes.TrimSpace(data)
	start := bytes.IndexByte(data, '{')
	if start < 0 {
		return ""
	}
	var u geforceNowUserJSON
	if err := json.Unmarshal(data[start:], &u); err == nil {
		if v := strings.TrimSpace(u.PreferredUsername); v != "" {
			return v
		}
	}
	return geforcePreferredUsernameFallback(data)
}

func geforcePreferredUsernameFallback(data []byte) string {
	const key = `"preferred_username":"`
	i := bytes.Index(data, []byte(key))
	if i < 0 {
		return ""
	}
	rest := data[i+len(key):]
	var b strings.Builder
	for pos := 0; pos < len(rest); pos++ {
		c := rest[pos]
		if c == '\\' && pos+1 < len(rest) {
			b.WriteByte(rest[pos+1])
			pos++
			continue
		}
		if c == '"' {
			break
		}
		b.WriteByte(c)
	}
	return strings.TrimSpace(b.String())
}
