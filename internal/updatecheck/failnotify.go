package updatecheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func failPath(exeDir string) string {
	return filepath.Join(exeDir, failStateFile)
}

func shouldEmitFailToast(exeDir string) bool {
	if exeDir == "" {
		return true
	}
	data, err := os.ReadFile(failPath(exeDir))
	if err != nil || len(data) == 0 {
		return true
	}
	var st failStateJSON
	if json.Unmarshal(data, &st) != nil || strings.TrimSpace(st.At) == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339Nano, st.At)
	if err != nil {
		t, err = time.Parse("2006-01-02 15:04:05.000", st.At)
	}
	if err != nil {
		return true
	}
	return time.Since(t) >= 24*time.Hour
}

func writeFailTimestamp(exeDir string) error {
	if exeDir == "" {
		return nil
	}
	st := failStateJSON{At: time.Now().Format(time.RFC3339Nano)}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(failPath(exeDir), data, 0o644)
}
