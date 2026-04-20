package platform

import (
	"os"
	"path/filepath"
	"strings"
)

// expandWindowsPath expands %Var% placeholders used in Platforms.json (Windows-style env vars).
func expandWindowsPath(s string) string {
	if s == "" {
		return ""
	}
	m := map[string]string{
		"%ProgramFiles%":         os.Getenv("ProgramFiles"),
		"%ProgramFiles(x86)%":    os.Getenv("ProgramFiles(x86)"),
		"%LocalAppData%":         os.Getenv("LocalAppData"),
		"%AppData%":              os.Getenv("AppData"),
		"%UserProfile%":          os.Getenv("UserProfile"),
		"%Desktop%":              filepath.Join(os.Getenv("USERPROFILE"), "Desktop"),
		"%StartMenuAppData%":     filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs`),
		"%StartMenuProgramData%": filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs`),
	}
	out := s
	for k, v := range m {
		if v != "" {
			out = strings.ReplaceAll(out, k, v)
		}
	}
	return filepath.Clean(out)
}
