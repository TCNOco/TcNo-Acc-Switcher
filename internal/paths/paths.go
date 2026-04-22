// Package paths resolves the app's data directory layout (next to exe for now).
package paths

import (
	"path/filepath"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
)

const DataDirName = "TcNo Account Switcher"

// DataRoot returns {ExeDir}/TcNo Account Switcher/
func DataRoot() (string, error) {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(exeDir, DataDirName), nil
}

func SettingsDir() (string, error) {
	r, err := DataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(r, "Settings"), nil
}

func SanitizePathSegment(name string) string {
	s := strings.TrimSpace(name)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 32:
			b.WriteRune('_')
		case r == '<' || r == '>' || r == ':' || r == '"' || r == '/' || r == '\\' || r == '|' || r == '?' || r == '*':
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimRight(b.String(), " .")
	out = strings.TrimSpace(out)
	for strings.Contains(out, "__") {
		out = strings.ReplaceAll(out, "__", "_")
	}
	if out == "" || out == "." || out == ".." {
		return ""
	}
	if windowsReservedFileStem(out) {
		return out + "_"
	}
	return out
}

func windowsReservedFileStem(s string) bool {
	u := strings.ToUpper(strings.TrimSpace(s))
	switch u {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(u) == 4 {
		c := u[3]
		if c >= '0' && c <= '9' {
			if strings.HasPrefix(u, "COM") || strings.HasPrefix(u, "LPT") {
				return true
			}
		}
	}
	return false
}

func LoginCacheDir(platformKey string) (string, error) {
	r, err := DataRoot()
	if err != nil {
		return "", err
	}
	seg := SanitizePathSegment(platformKey)
	if seg == "" {
		seg = "platform"
	}
	return filepath.Join(r, "LoginCache", seg), nil
}

func WwwrootDir() (string, error) {
	r, err := DataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(r, "wwwroot"), nil
}
