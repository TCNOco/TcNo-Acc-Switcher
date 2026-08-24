package steam

import (
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/security"
)

// Switching an account closes Steam, and in Game Mode the switcher is inside
// Steam's process tree, so closing Steam kills the switcher before it can write
// the account it was asked to switch to. Measured on an ROG Ally: Steam came
// back on the old account every time, restarted by
// gamescope-session-plus@steam.service rather than by us, because we were gone.
//
// So in Game Mode the swap is handed to a process the user's systemd manager
// starts. That one is outside Steam's reach and lives long enough to finish -
// verified on the same device, where a transient unit survived a full Steam
// restart with both its timestamps intact.
const (
	// switchHelperEnv marks the helper, so it performs the swap rather than
	// handing it off again.
	switchHelperEnv = "TCNO_SWITCH_HELPER"

	// gameModeSwitchFlag is the argv token that tells the helper it is the
	// helper. It has to match the flag internal/cli parses.
	gameModeSwitchFlag = "--gamescope-switch"

	// handoffLinger is how long the caller waits to be taken down with Steam
	// after handing over. It only has to outlast the helper closing Steam.
	handoffLinger = 30 * time.Second
)

// ErrGameModeSwitchLocked reports that Game Mode cannot switch while an app
// password is set, because the helper starts as a new process with the vault
// closed and nothing may open it on the user's behalf.
var ErrGameModeSwitchLocked = errors.New("switching from Game Mode is not available while an app password is set")

// MarkSwitchHelper records that this process was started to perform a swap.
// Called from main before anything reads it.
func MarkSwitchHelper() { _ = os.Setenv(switchHelperEnv, "1") }

func runningAsSwitchHelper() bool { return strings.TrimSpace(os.Getenv(switchHelperEnv)) != "" }

// shouldHandOffSwitch is the whole decision: hand the swap to a helper only in a
// gamescope session, and only from a process that is not already that helper.
func shouldHandOffSwitch(inGamescope, isHelper bool) bool { return inGamescope && !isHelper }

// switchHelperArgs is the helper's argv. It speaks the CLI grammar the headless
// swap already understands, so the helper runs the same code a scripted switch
// does rather than a second implementation of it.
func switchHelperArgs(steamID64 string, personaState int) []string {
	id := strings.TrimSpace(steamID64)
	if id == "" {
		return []string{"logout:Steam", gameModeSwitchFlag}
	}
	if personaState < 0 {
		return []string{"+s:" + id, gameModeSwitchFlag}
	}
	return []string{"+s:" + id + ":" + strconv.Itoa(personaState), gameModeSwitchFlag}
}

// handOffSwitch starts the helper and then waits to be killed along with Steam.
//
// It deliberately does not exit by itself. On a session whose shell is not Steam
// the client's exit takes nothing else down, and quitting here would close the
// app for no reason; waiting means the window stays up until whatever is going
// to happen has happened.
func handOffSwitch(steamID64 string, personaState int) error {
	// The helper is a fresh process with the vault closed, and nothing may open
	// it without the user. Better to say so now than to hand over and have the
	// switch fail somewhere the user cannot see it.
	if status, err := security.GetStatus(); err == nil && status.AppPasswordSet {
		return ErrGameModeSwitchLocked
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := switchHelperArgs(steamID64, personaState)
	slog.Info("game mode: handing the switch to a helper outside Steam's process tree", "args", args)
	if err := startSwitchHelper(exe, args); err != nil {
		return err
	}

	platform.EmitActionBarStatusI18nPlatform("Status_ClosingPlatform", "Steam")
	waitToBeClosedWithSteam()
	return nil
}

// waitToBeClosedWithSteam parks until Steam is gone, which is when the helper
// has done its part and this process is about to be taken down with it.
func waitToBeClosedWithSteam() {
	deadline := time.Now().Add(handoffLinger)
	for time.Now().Before(deadline) {
		if !steamIsRunning() {
			// Steam has gone; the reaper that owns this process goes with it.
			// Anything still running here is on borrowed time.
			time.Sleep(2 * time.Second)
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	slog.Warn("game mode: still running after handing the switch over", "waited", handoffLinger)
}

// startsSteamAfterSwap reports whether the swap should start Steam itself.
//
// Never in Game Mode: the session unit supervises Steam and restarts it on its
// own, and a second Steam racing that one is worse than no Steam at all.
func startsSteamAfterSwap(autoStart, gameMode bool) bool { return autoStart && !gameMode }
