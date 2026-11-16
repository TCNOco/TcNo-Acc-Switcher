package platforms

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// CachedJSONValue selects a string field from complete JSON objects embedded in
// cache blobs. Optional matching confines the result to the current account.
// File discovery, recency ordering and shared reads use the common cache reader.
func CachedJSONValue(pattern string, fallbackPatterns []string, jsonPath, matchPath, matchValue string) (string, error) {
	jsonPath = strings.TrimSpace(jsonPath)
	matchPath = strings.TrimSpace(matchPath)
	matchValue = strings.TrimSpace(matchValue)
	if jsonPath == "" {
		return "", fmt.Errorf("cached JSON selection requires a JSON path")
	}
	if matchPath != "" && matchValue == "" {
		return "", fmt.Errorf("cached JSON matching requires the current account ID")
	}
	return firstCacheValue(pattern, fallbackPatterns, func(data []byte) string {
		return parseCachedJSONValue(data, jsonPath, matchPath, matchValue)
	})
}

func parseCachedJSONValue(data []byte, jsonPath, matchPath, matchValue string) string {
	for offset := 0; offset < len(data); {
		rel := bytes.IndexByte(data[offset:], '{')
		if rel < 0 {
			break
		}
		start := offset + rel
		decoder := json.NewDecoder(bytes.NewReader(data[start:]))
		var object json.RawMessage
		if err := decoder.Decode(&object); err != nil {
			// Binary headers and truncated cache records can contain stray braces.
			offset = start + 1
			continue
		}
		offset = start + int(decoder.InputOffset())
		if matchPath != "" {
			id := gjson.GetBytes(object, matchPath)
			if id.Type != gjson.String || id.String() != matchValue {
				continue
			}
		}
		value := gjson.GetBytes(object, jsonPath)
		if value.Type == gjson.String {
			if name := strings.TrimSpace(value.String()); name != "" {
				return name
			}
		}
	}
	return ""
}
