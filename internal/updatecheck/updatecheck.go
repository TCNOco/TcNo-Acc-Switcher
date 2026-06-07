package updatecheck

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/api"
)

const (
	failStateFile    = "TcNo-Acc-Switcher.lastUpdateCheckFail.json"
	httpTimeout      = 15 * time.Second
	launchCheckDelay = 700 * time.Millisecond
)

var (
	launchOnce       sync.Once
	sharedHTTPClient = &http.Client{Timeout: httpTimeout}
)

type failStateJSON struct {
	At string `json:"at"`
}

// ParseVersionClock parses a semver version string (e.g. "4.0.0" or "v4.0.0")
// into a comparable time.Time. MAJOR.MINOR.PATCH maps to year.month.day.
func ParseVersionClock(s string) (time.Time, error) {
	s = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "\r", ""), "\n", ""))
	s = strings.TrimPrefix(s, "v")

	var y, m, d int
	if n, _ := fmt.Sscanf(s, "%d.%d.%d", &y, &m, &d); n == 3 {
		return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC), nil
	}
	return time.Time{}, fmt.Errorf("updatecheck: invalid semver format: %q", s)
}

func IsUpToDate(current, latest string) bool {
	cur, errC := ParseVersionClock(current)
	lat, errL := ParseVersionClock(latest)
	if errC != nil || errL != nil {
		return false
	}
	return lat.Equal(cur) || cur.After(lat)
}

func FetchLatestVersion(ctx context.Context, client *http.Client, currentVersion string) (version string, message string, err error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, updateAPIURL(currentVersion), nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", api.UserAgent(strings.TrimSpace(currentVersion)))
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("updatecheck: HTTP %d", resp.StatusCode)
	}
	lines := strings.SplitN(strings.TrimSpace(string(body)), "\n", 2)
	version = strings.TrimSpace(lines[0])
	if len(lines) > 1 {
		message = strings.TrimSpace(lines[1])
	}
	return version, message, nil
}

func StartLaunchCheck(exeDir string, offline bool, currentVersion string, onUpdateAvailable func(message string), onCheckFailed func()) {
	launchOnce.Do(func() {
		go runLaunchCheck(exeDir, offline, currentVersion, onUpdateAvailable, onCheckFailed)
	})
}

func runLaunchCheck(exeDir string, offline bool, currentVersion string, onUpdateAvailable func(message string), onCheckFailed func()) {
	time.Sleep(launchCheckDelay)
	if offline {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()

	latest, message, err := FetchLatestVersion(ctx, sharedHTTPClient, currentVersion)
	if err != nil {
		if shouldEmitFailToast(exeDir) {
			_ = writeFailTimestamp(exeDir)
			if onCheckFailed != nil {
				onCheckFailed()
			}
		}
		return
	}
	if IsUpToDate(currentVersion, latest) {
		return
	}
	if onUpdateAvailable != nil {
		onUpdateAvailable(message)
	}
}
