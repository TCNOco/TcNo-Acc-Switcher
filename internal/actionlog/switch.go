package actionlog

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/logredact"
)

const maxSwitchAttempts = 5
const maxSwitchEvents = 128

// SwitchAttempt retains a bounded diagnostic trace independently of background
// file activity. It records operations, never claims Steam accepted a login.
// All fields are protected by the action log mutex.
type SwitchAttempt struct {
	id           int
	started      time.Time
	stage        string
	stageStarted time.Time
	events       []string
	accounts     [][]string
	finished     bool
	omitted      int
}

var switchAttempts []*SwitchAttempt
var nextSwitchID int

// BeginSwitch captures a target identifier for upload-time account aliasing.
// Do not pass credentials, raw launch arguments, or file contents as details.
func BeginSwitch(platform, target string) *SwitchAttempt {
	mu.Lock()
	defer mu.Unlock()
	nextSwitchID++
	now := time.Now()
	a := &SwitchAttempt{id: nextSwitchID, started: now}
	if target != "" {
		a.accounts = append(a.accounts, []string{target})
	}
	a.appendLocked("begin", "started", 0, map[string]any{
		"platform": platform, "target": target, "os": runtime.GOOS, "arch": runtime.GOARCH,
		"revision": buildRevision(),
	}, nil)
	if ready {
		switchAttempts = append(switchAttempts, a)
		if len(switchAttempts) > maxSwitchAttempts {
			switchAttempts = switchAttempts[1:]
		}
	}
	return a
}

// Account registers identifiers while they are still available, including
// accounts whose VDF/cache entries may disappear before the user submits a report.
func (a *SwitchAttempt) Account(ids ...string) {
	mu.Lock()
	defer mu.Unlock()
	if len(a.accounts) < 256 {
		a.accounts = append(a.accounts, append([]string(nil), ids...))
	}
}

// Next records completion of the previous stage and starts the next one.
func (a *SwitchAttempt) Next(stage string) {
	mu.Lock()
	defer mu.Unlock()
	if a.finished {
		return
	}
	now := time.Now()
	if a.stage != "" {
		a.appendLocked(a.stage, "ok", now.Sub(a.stageStarted), nil, nil)
	}
	a.stage, a.stageStarted = stage, now
	a.appendLocked(stage, "started", 0, nil, nil)
}

// Note adds safe metadata to the current stage.
func (a *SwitchAttempt) Note(details map[string]any) {
	mu.Lock()
	defer mu.Unlock()
	if !a.finished {
		a.appendLocked(a.stage, "info", 0, details, nil)
	}
}

// Warning records a nonfatal error that would otherwise be absent from reports.
func (a *SwitchAttempt) Warning(operation string, err error) {
	if err == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if !a.finished {
		a.appendLocked(operation, "warning", 0, nil, err)
	}
}

// Finish records the actual operation result, independently of stability counters.
func (a *SwitchAttempt) Finish(result string, err error) {
	mu.Lock()
	defer mu.Unlock()
	if a.finished {
		return
	}
	outcome := "ok"
	if result == "incomplete" {
		outcome = "unknown"
	}
	if err != nil {
		outcome = "fail"
		result = "failed"
	}
	now := time.Now()
	if a.stage != "" {
		a.appendLocked(a.stage, outcome, now.Sub(a.stageStarted), nil, err)
	}
	a.appendLocked("end", outcome, now.Sub(a.started), map[string]any{"result": result, "login_verified": false}, err)
	a.finished = true
}

func (a *SwitchAttempt) appendLocked(stage, outcome string, elapsed time.Duration, details map[string]any, err error) {
	line := fmt.Sprintf("%s switch=%d stage=%s outcome=%s duration_ms=%d",
		time.Now().UTC().Format(time.RFC3339Nano), a.id, stage, outcome, elapsed.Milliseconds())
	if details != nil {
		line += fmt.Sprintf(" detail=%q", logredact.FormatValue(details))
	}
	if err != nil {
		line += fmt.Sprintf(" err=%q", logredact.FormatValue(err))
	}
	line = logredact.RedactText(line)
	if len(a.events) >= maxSwitchEvents {
		// Retain the attempt's context and its most recent stages, including its end.
		a.events = append(a.events[:1], a.events[2:]...)
		a.omitted++
	}
	a.events = append(a.events, line)
}

// SwitchSnapshot returns retained attempts and their identifiers atomically.
// The caller must apply account redaction before displaying or uploading text.
func SwitchSnapshot() (string, [][]string) {
	mu.Lock()
	defer mu.Unlock()
	var out strings.Builder
	var accounts [][]string
	for _, a := range switchAttempts {
		for _, ids := range a.accounts {
			accounts = append(accounts, append([]string(nil), ids...))
		}
		for _, line := range a.events {
			out.WriteString(line)
			out.WriteByte('\n')
		}
		if a.omitted > 0 {
			fmt.Fprintf(&out, "switch=%d omitted_events=%d\n", a.id, a.omitted)
		}
	}
	return strings.TrimSpace(out.String()), accounts
}

func buildRevision() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		revision := "unknown"
		dirty := false
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				revision = setting.Value
			}
			if setting.Key == "vcs.modified" {
				dirty = setting.Value == "true"
			}
		}
		if dirty {
			revision += "+modified"
		}
		return revision
	}
	return "unknown"
}
