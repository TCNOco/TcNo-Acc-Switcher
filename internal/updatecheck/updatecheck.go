package updatecheck

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	failStateFile    = "TcNo-Acc-Switcher.lastUpdateCheckFail.json"
	httpTimeout      = 15 * time.Second
	launchCheckDelay = 700 * time.Millisecond
)

var launchOnce sync.Once

type failStateJSON struct {
	At string `json:"at"`
}

// ParseVersionClock parses a version string like 2026-05-04_00
func ParseVersionClock(s string) (time.Time, error) {
	s = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "\r", ""), "\n", ""))
	parts := strings.SplitN(s, "_", 2)
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("updatecheck: missing underscore suffix")
	}
	day, err := time.ParseInLocation("2006-01-02", parts[0], time.Local)
	if err != nil {
		return time.Time{}, err
	}
	min, err := strconv.Atoi(parts[1])
	if err != nil || min < 0 || min > 59 {
		return time.Time{}, fmt.Errorf("updatecheck: suffix not in 0..59")
	}
	return day.Add(time.Duration(min) * time.Minute), nil
}

func IsUpToDate(current, latest string) bool {
	cur, errC := ParseVersionClock(current)
	lat, errL := ParseVersionClock(latest)
	if errC != nil || errL != nil {
		return false
	}
	return lat.Equal(cur) || cur.After(lat)
}

func FetchLatestVersion(ctx context.Context, client *http.Client, currentVersion string) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, updateAPIURL(currentVersion), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "TcNo-Acc-Switcher/"+strings.TrimSpace(currentVersion))
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("updatecheck: HTTP %d", resp.StatusCode)
	}
	return strings.TrimSpace(string(body)), nil
}

func StartLaunchCheck(exeDir string, offline bool, currentVersion string, onUpdateAvailable, onCheckFailed func()) {
	launchOnce.Do(func() {
		go runLaunchCheck(exeDir, offline, currentVersion, onUpdateAvailable, onCheckFailed)
	})
}

func runLaunchCheck(exeDir string, offline bool, currentVersion string, onUpdateAvailable, onCheckFailed func()) {
	time.Sleep(launchCheckDelay)
	if offline {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()

	client := &http.Client{Timeout: httpTimeout}
	latest, err := FetchLatestVersion(ctx, client, currentVersion)
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
		onUpdateAvailable()
	}
}
