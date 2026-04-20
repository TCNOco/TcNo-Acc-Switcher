package windows

import (
	"encoding/base64"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ResolveLnkTarget returns the shortcut target path for a .lnk file.
func ResolveLnkTarget(lnkPath string) (string, error) {
	if lnkPath == "" {
		return "", fmt.Errorf("empty shortcut path")
	}
	b64 := base64.StdEncoding.EncodeToString([]byte(lnkPath))
	ps := fmt.Sprintf(
		"$p=[System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s')); (New-Object -ComObject WScript.Shell).CreateShortcut($p).TargetPath",
		b64,
	)
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-STA", "-Command", ps).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// OrderedLnkPathsForTitle finds Start Menu (user + common) and Desktop shortcuts named "{title}.lnk".
func OrderedLnkPathsForTitle(title string) []string {
	t := strings.TrimSpace(title)
	if t == "" {
		return nil
	}
	want := t + ".lnk"
	var out []string

	smApp := filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs`)
	smCommon := filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs`)
	desktop := filepath.Join(os.Getenv("USERPROFILE"), "Desktop")

	walk := func(root string) {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if strings.EqualFold(filepath.Base(path), want) {
				out = append(out, path)
			}
			return nil
		})
	}
	walk(smApp)
	walk(smCommon)

	if ents, err := os.ReadDir(desktop); err == nil {
		for _, e := range ents {
			if e.IsDir() {
				continue
			}
			if strings.EqualFold(e.Name(), want) {
				out = append(out, filepath.Join(desktop, e.Name()))
			}
		}
	}
	return out
}
