//go:build windows

package winutil

import (
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
)

// startupTaskFolder is the Task Scheduler folder holding this app's logon tasks.
const startupTaskFolder = `\TcNo Account Switcher`

// A Run entry always starts unelevated, so "start with Windows" combined with
// "always run as admin" only works as a scheduled task with a highest-available
// run level. Tasks are machine-wide while the preference is per-user, so the
// name carries the account it starts for.
func startupTrayTaskName(account string) string {
	account = sanitizeTaskNameComponent(account)
	if account == "" {
		return startupTaskFolder + `\Start Tray`
	}
	return startupTaskFolder + `\Start Tray - ` + account
}

// sanitizeTaskNameComponent strips the characters Task Scheduler rejects in a
// task name; backslashes matter most, as they would split the name into folders.
func sanitizeTaskNameComponent(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\', '/', ':', '*', '?', '"', '<', '>', '|':
			b.WriteByte('_')
		default:
			if r < 0x20 {
				continue
			}
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func currentUserForTask() (name, sid string, err error) {
	u, err := user.Current()
	if err != nil {
		return "", "", err
	}
	return u.Username, u.Uid, nil
}

// StartupTrayTaskExists reports whether the elevated logon task is registered for
// the current user. Querying needs no elevation, unlike creating and deleting.
func StartupTrayTaskExists() (bool, error) {
	name, _, err := currentUserForTask()
	if err != nil {
		return false, err
	}
	if _, err := runSchtasks("/Query", "/TN", startupTrayTaskName(name)); err != nil {
		if isTaskNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// SetStartupTrayTask registers or removes the logon task that starts the tray
// elevated. Both creating and deleting a HighestAvailable task require the
// current process to be elevated.
func SetStartupTrayTask(exePath string, enabled bool) error {
	name, sid, err := currentUserForTask()
	if err != nil {
		return err
	}
	taskName := startupTrayTaskName(name)

	if !enabled {
		if _, err := runSchtasks("/Delete", "/TN", taskName, "/F"); err != nil && !isTaskNotFound(err) {
			return err
		}
		return nil
	}

	exePath = filepath.Clean(strings.TrimSpace(exePath))
	if exePath == "" {
		return fmt.Errorf("empty executable path")
	}

	// The preference is reconciled on every launch, so skip the (elevation-
	// requiring) write when the registered task already points at this build.
	if cur, ok := registeredStartupTrayTaskCommand(taskName); ok && strings.EqualFold(cur, exePath) {
		return nil
	}

	xmlPath, err := writeStartupTrayTaskXML(exePath, sid)
	if err != nil {
		return err
	}
	defer os.Remove(xmlPath)

	_, err = runSchtasks("/Create", "/TN", taskName, "/XML", xmlPath, "/F")
	return err
}

// registeredStartupTrayTaskCommand returns the executable the registered task
// runs. ok is false when there is no task, or its definition cannot be read.
func registeredStartupTrayTaskCommand(taskName string) (cmd string, ok bool) {
	out, err := runSchtasks("/Query", "/TN", taskName, "/XML", "ONE")
	if err != nil {
		return "", false
	}
	var def struct {
		Actions struct {
			Exec []struct {
				Command string `xml:"Command"`
			} `xml:"Exec"`
		} `xml:"Actions"`
	}
	dec := xml.NewDecoder(strings.NewReader(out))
	// The declaration says UTF-16; runSchtasks already decoded the bytes, so the
	// reader hands the decoder the UTF-8 it actually wants.
	dec.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
	if err := dec.Decode(&def); err != nil || len(def.Actions.Exec) == 0 {
		return "", false
	}
	// Task Scheduler keeps whatever quoting the definition was registered with.
	c := strings.Trim(strings.TrimSpace(def.Actions.Exec[0].Command), `"`)
	if c == "" {
		return "", false
	}
	return filepath.Clean(c), true
}

func writeStartupTrayTaskXML(exePath, userID string) (string, error) {
	f, err := os.CreateTemp("", "tcno-startup-task-*.xml")
	if err != nil {
		return "", err
	}
	path := f.Name()
	_, err = f.Write(utf16LEWithBOM(startupTrayTaskXML(exePath, userID)))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

// startupTrayTaskXML builds the Task Scheduler definition. It is written out in
// full rather than assembled from /Create flags because those default to giving
// the task a three-day time limit and refusing to start on battery, neither of
// which suits something that lives in the tray.
func startupTrayTaskXML(exePath, userID string) string {
	return `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>Starts TcNo Account Switcher in the tray, as administrator, when you sign in.</Description>
  </RegistrationInfo>
  <Triggers>
    <LogonTrigger>
      <Enabled>true</Enabled>
      <UserId>` + escapeXML(userID) + `</UserId>
    </LogonTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author">
      <UserId>` + escapeXML(userID) + `</UserId>
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>HighestAvailable</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>false</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <IdleSettings>
      <StopOnIdleEnd>false</StopOnIdleEnd>
      <RestartOnIdle>false</RestartOnIdle>
    </IdleSettings>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <Hidden>false</Hidden>
    <RunOnlyIfIdle>false</RunOnlyIfIdle>
    <WakeToRun>false</WakeToRun>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>7</Priority>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>` + escapeXML(exePath) + `</Command>
      <Arguments>` + escapeXML(RunCommandTrayArgs) + `</Arguments>
      <WorkingDirectory>` + escapeXML(filepath.Dir(exePath)) + `</WorkingDirectory>
    </Exec>
  </Actions>
</Task>`
}

func escapeXML(s string) string {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(s)); err != nil {
		return ""
	}
	return b.String()
}

// schtasks only reads task XML as UTF-16; a UTF-8 file is reported as malformed.
func utf16LEWithBOM(s string) []byte {
	units := utf16.Encode([]rune(s))
	out := make([]byte, 0, 2+len(units)*2)
	out = append(out, 0xFF, 0xFE)
	var buf [2]byte
	for _, u := range units {
		binary.LittleEndian.PutUint16(buf[:], u)
		out = append(out, buf[0], buf[1])
	}
	return out
}

type schtasksError struct {
	err    error
	output string
}

func (e *schtasksError) Error() string {
	if e.output == "" {
		return fmt.Sprintf("schtasks: %v", e.err)
	}
	return fmt.Sprintf("schtasks: %v: %s", e.err, e.output)
}

func (e *schtasksError) Unwrap() error { return e.err }

func runSchtasks(args ...string) (string, error) {
	cmd := exec.Command("schtasks.exe", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(decodeSchtasksOutput(out))
	if err != nil {
		return text, &schtasksError{err: err, output: text}
	}
	return text, nil
}

// decodeSchtasksOutput converts the /XML output, which schtasks writes as
// UTF-16LE with a BOM when redirected, to a Go string. Everything else it prints
// is already single-byte text.
func decodeSchtasksOutput(b []byte) string {
	if len(b) < 2 || b[0] != 0xFF || b[1] != 0xFE {
		return string(b)
	}
	b = b[2:]
	units := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		units = append(units, binary.LittleEndian.Uint16(b[i:i+2]))
	}
	return string(utf16.Decode(units))
}

// isTaskNotFound distinguishes "there is no such task" - the expected answer when
// querying or removing a task that was never registered - from a real failure.
func isTaskNotFound(err error) bool {
	var se *schtasksError
	if !errors.As(err, &se) {
		return false
	}
	s := strings.ToLower(se.output)
	return strings.Contains(s, "cannot find the file specified") ||
		strings.Contains(s, "does not exist") ||
		strings.Contains(s, "cannot find the path specified")
}
