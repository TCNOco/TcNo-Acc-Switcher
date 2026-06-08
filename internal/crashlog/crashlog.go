package crashlog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	buildinfo "TcNo-Acc-Switcher/build"
	"TcNo-Acc-Switcher/internal/api"
	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/stats"
)

const (
	crashDumpFile = "CrashDump.json"
)

type CrashDump struct {
	Stack     string `json:"stack"`
	Error     string `json:"error"`
	Version   string `json:"version"`
	OS        string `json:"os"`
	Timestamp string `json:"timestamp"`
	UUID      string `json:"uuid"`
}

func exeDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

func crashDumpPath() (string, error) {
	dir, err := exeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, crashDumpFile), nil
}

const settingsFileName = "TcNo-Acc-Switcher.settings.json"

func readSettingsValue(key string) string {
	dir, err := exeDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(dir, settingsFileName))
	if err != nil {
		return ""
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	v, ok := raw[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return ""
	}
	return s
}

func isOfflineMode() bool {
	return readSettingsValue("offlineMode") == "true"
}

func readStatsUUID() string {
	root, err := paths.DataRoot()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(root, "Statistics.json"))
	if err != nil {
		return ""
	}
	var aux struct {
		Uuid string `json:"Uuid"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return ""
	}
	return aux.Uuid
}

func writeCrashDump(dump CrashDump) error {
	path, err := crashDumpPath()
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, payload, 0o644)
}

// Capture recovers from a panic, logs it, writes CrashDump.json, and exits.
func Capture() {
	r := recover()
	if r == nil {
		return
	}

	stack := string(debug.Stack())
	slog.Error("panic recovered",
		"error", r,
		"stack", stack,
	)

	dump := CrashDump{
		Stack:     stack,
		Error:     fmt.Sprint(r),
		Version:   buildinfo.Version(),
		OS:        runtime.GOOS + "/" + runtime.GOARCH,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		UUID:      readStatsUUID(),
	}

	if err := writeCrashDump(dump); err != nil {
		slog.Warn("writing crash dump", "err", err)
	}

	if err := stats.IncrementCrashCount(); err != nil {
		slog.Warn("incrementing crash count", "err", err)
	}

	os.Exit(1)
}

// SubmitPending checks for a crash dump from a previous run and submits it.
// Returns true if a crash dump was found and successfully submitted.
func SubmitPending() bool {
	if isOfflineMode() {
		return false
	}

	path, err := crashDumpPath()
	if err != nil {
		slog.Warn("crashlog: resolving dump path", "err", err)
		return false
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		slog.Warn("crashlog: reading pending dump", "err", err)
		return false
	}

	req, err := http.NewRequest(http.MethodPost, api.CrashURL(), bytes.NewReader(data))
	if err != nil {
		slog.Warn("crashlog: building submit request", "err", err)
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", api.UserAgent(buildinfo.Version()))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("crashlog: submitting dump", "err", err)
		return false
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("crashlog: submission rejected", "status", resp.StatusCode)
		return false
	}

	if err := os.Remove(path); err != nil {
		slog.Warn("crashlog: removing submitted dump", "err", err)
	}

	return true
}
