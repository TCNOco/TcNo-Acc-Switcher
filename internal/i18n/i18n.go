package i18n

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	cacheMu sync.Mutex
	cache   = map[string]map[string]string{}
)

// T returns a localized string from the existing frontend resource JSON files.
// It is intentionally small for native Go surfaces such as the tray menu.
func T(exeDir, language, key string, vars map[string]string) string {
	language = strings.TrimSpace(language)
	if language == "" {
		language = "en-US"
	}
	messages := loadMessages(exeDir, language)
	if language != "en-US" {
		en := loadMessages(exeDir, "en-US")
		merged := make(map[string]string, len(en)+len(messages))
		for k, v := range en {
			merged[k] = v
		}
		for k, v := range messages {
			merged[k] = v
		}
		messages = merged
	}
	template := messages[key]
	if template == "" {
		template = key
	}
	for k, v := range vars {
		template = strings.ReplaceAll(template, "{"+k+"}", v)
	}
	return template
}

func loadMessages(exeDir, language string) map[string]string {
	cacheKey := exeDir + "\x00" + language
	cacheMu.Lock()
	if v, ok := cache[cacheKey]; ok {
		cacheMu.Unlock()
		return v
	}
	cacheMu.Unlock()

	messages := readMessages(exeDir, language)
	cacheMu.Lock()
	cache[cacheKey] = messages
	cacheMu.Unlock()
	return messages
}

func readMessages(exeDir, language string) map[string]string {
	for _, base := range resourceSearchRoots(exeDir) {
		p := filepath.Join(base, "frontend", "src", "Resources", language+".json")
		if messages, ok := readResourceFile(p); ok {
			return messages
		}
		p = filepath.Join(base, "Resources", language+".json")
		if messages, ok := readResourceFile(p); ok {
			return messages
		}
	}
	return map[string]string{}
}

func resourceSearchRoots(exeDir string) []string {
	seen := map[string]struct{}{}
	var roots []string
	addAncestors := func(start string) {
		start = strings.TrimSpace(start)
		if start == "" {
			return
		}
		dir, err := filepath.Abs(start)
		if err != nil {
			dir = start
		}
		for {
			if _, ok := seen[dir]; !ok {
				seen[dir] = struct{}{}
				roots = append(roots, dir)
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				return
			}
			dir = parent
		}
	}
	addAncestors(exeDir)
	if wd, err := os.Getwd(); err == nil {
		addAncestors(wd)
	}
	return roots
}

func readResourceFile(path string) (map[string]string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var messages map[string]string
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, false
	}
	return messages, true
}
