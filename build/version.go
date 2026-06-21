package buildinfo

import (
	_ "embed"
	"strings"
)

//go:embed config.yml
var embeddedBuildConfig string

// Version returns info.version from build/config.yml embedded at compile-time.
func Version() string {
	lines := strings.Split(embeddedBuildConfig, "\n")
	inInfo := false
	infoIndent := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "info:") {
			inInfo = true
			infoIndent = indent
			continue
		}
		if inInfo && indent <= infoIndent {
			inInfo = false
		}
		if !inInfo || !strings.HasPrefix(trimmed, "version:") {
			continue
		}
		v := strings.TrimSpace(strings.TrimPrefix(trimmed, "version:"))
		if i := strings.Index(v, "#"); i >= 0 {
			v = strings.TrimSpace(v[:i])
		}
		v = strings.Trim(v, `"'`)
		if v != "" {
			return v
		}
	}
	return "0.0.0"
}
