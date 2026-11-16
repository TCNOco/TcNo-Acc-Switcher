package basic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"TcNo-Acc-Switcher/internal/fsutil"

	"github.com/tidwall/sjson"
	"go.yaml.in/yaml/v3"
)

// applyConfigOverrides uses only launcher-owned files already covered by LoginFiles.
// Missing files are left for the launcher to create on its first launch.
func applyConfigOverrides(fc FlowContext) error {
	for _, override := range fc.Descriptor.Extras.ConfigOverrides {
		path := expandPlatformPath(override.File, fc.Folder, fc.PathCtx)
		if err := validateCredentialSessionPath(path); err != nil {
			return fmt.Errorf("config override path: %w", err)
		}
		if !configOverrideOwned(fc, path) {
			return fmt.Errorf("config override file is not included in LoginFiles: %s", path)
		}
		data, err := readCredentialSessionFile(path, maxDescriptorJSONFileSize)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read config override file %s: %w", path, err)
		}
		patched, changed, err := patchConfigValue(data, override.Format, override.Path, override.Value)
		if err != nil {
			return fmt.Errorf("config override %s (%s): %w", path, override.Path, err)
		}
		if changed {
			if err := fsutil.WriteFileAtomic(path, patched, 0o600); err != nil {
				return fmt.Errorf("write config override %s: %w", path, err)
			}
			logFlow().Debug("applied config override", "file", path, "path", override.Path)
		}
	}
	return nil
}

func configOverrideOwned(fc FlowContext, path string) bool {
	for raw := range fc.Descriptor.LoginFiles {
		live := expandPlatformPath(raw, fc.Folder, fc.PathCtx)
		if sameSessionPath(path, live) {
			return true
		}
		if hasGlobPattern(live) {
			if matches, err := filepath.Match(live, path); err == nil && matches {
				return true
			}
			continue
		}
		if info, err := os.Stat(live); err == nil && info.IsDir() {
			rel, err := filepath.Rel(live, path)
			if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return true
			}
		}
	}
	return false
}

// patchConfigValue changes one scalar using a dotted mapping path. Unchanged
// values return the original bytes, avoiding rewrites of a running app's config.
func patchConfigValue(data []byte, format, path string, value any) ([]byte, bool, error) {
	parts := strings.Split(path, ".")
	for _, part := range parts {
		if strings.TrimSpace(part) == "" || strings.ContainsAny(part, `*?[]#\`) {
			return nil, false, fmt.Errorf("config path requires plain mapping keys separated by dots")
		}
	}
	var replacement yaml.Node
	if err := replacement.Encode(value); err != nil || replacement.Kind != yaml.ScalarNode {
		return nil, false, fmt.Errorf("config override requires a scalar value")
	}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		var current any
		if err := json.Unmarshal(data, &current); err != nil {
			return nil, false, fmt.Errorf("invalid JSON config")
		}
		for i, part := range parts {
			mapping, ok := current.(map[string]any)
			if !ok {
				return nil, false, fmt.Errorf("config path crosses a non-mapping value")
			}
			next, exists := mapping[part]
			if i == len(parts)-1 {
				switch next.(type) {
				case map[string]any, []any:
					return nil, false, fmt.Errorf("config override target must be a scalar")
				}
				if exists && reflect.DeepEqual(next, value) {
					return data, false, nil
				}
			}
			if exists {
				current = next
			} else {
				current = map[string]any{}
			}
		}
		patched, err := sjson.SetBytes(data, path, value)
		return patched, err == nil && !bytes.Equal(patched, data), err
	case "yaml", "yml":
		var doc yaml.Node
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		if err := decoder.Decode(&doc); err != nil {
			return nil, false, err
		}
		var extra yaml.Node
		if err := decoder.Decode(&extra); err != io.EOF {
			return nil, false, fmt.Errorf("config must contain one YAML document")
		}
		if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
			return nil, false, fmt.Errorf("YAML config root must be a mapping")
		}
		node := doc.Content[0]
		for i, part := range parts {
			if node.Kind != yaml.MappingNode {
				return nil, false, fmt.Errorf("config path crosses a non-mapping value")
			}
			var next *yaml.Node
			for j := 0; j < len(node.Content); j += 2 {
				if node.Content[j].Value == part {
					if next != nil {
						return nil, false, fmt.Errorf("config path contains a duplicate key")
					}
					next = node.Content[j+1]
				}
			}
			created := next == nil
			if created {
				next = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
				node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: part}, next)
			}
			if i == len(parts)-1 {
				if !created && next.Kind != yaml.ScalarNode {
					return nil, false, fmt.Errorf("config override target must be a scalar")
				}
				if next.Kind == yaml.ScalarNode && next.Tag == replacement.Tag && next.Value == replacement.Value {
					return data, false, nil
				}
				replacement.HeadComment, replacement.LineComment, replacement.FootComment = next.HeadComment, next.LineComment, next.FootComment
				*next = replacement
				break
			}
			node = next
		}
		var out bytes.Buffer
		encoder := yaml.NewEncoder(&out)
		encoder.SetIndent(2)
		if err := encoder.Encode(&doc); err != nil {
			return nil, false, err
		}
		if err := encoder.Close(); err != nil {
			return nil, false, err
		}
		return out.Bytes(), true, nil
	default:
		return nil, false, fmt.Errorf("unsupported config format %q", format)
	}
}
