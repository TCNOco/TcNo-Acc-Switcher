//go:build linux

package steam

import (
	"fmt"
	"os"
	"os/exec"
)

// sessionEnvForRelaunch is what a graphical app cannot start without. A transient
// systemd unit is started by the user's manager, so it inherits that manager's
// environment rather than ours; desktops that run `systemctl --user
// import-environment` have put these there already, and passing them explicitly
// covers the ones that have not.
var sessionEnvForRelaunch = []string{
	"DISPLAY",
	"WAYLAND_DISPLAY",
	"XAUTHORITY",
	"XDG_RUNTIME_DIR",
	"XDG_SESSION_TYPE",
	"XDG_CURRENT_DESKTOP",
	"DBUS_SESSION_BUS_ADDRESS",
	"LANG",
}

// systemdRunOutsideSteam starts exe under the user's systemd manager, which is
// the only process that can start something Steam's subreaper cannot follow.
// Forking is no use: the reaper is a subreaper, so an orphan reparents onto it
// rather than to init, and dies with Steam anyway.
//
// setEnv is set on the unit explicitly. forwardEnv names variables to copy from
// this process when they have a value - a transient unit inherits the manager's
// environment, not ours.
func systemdRunOutsideSteam(exe string, args []string, setEnv map[string]string, forwardEnv []string) error {
	systemdRun, err := exec.LookPath("systemd-run")
	if err != nil {
		return fmt.Errorf("systemd-run is needed to start outside Steam's process tree: %w", err)
	}

	// No --unit: letting systemd name the transient unit means a switcher that is
	// already running cannot collide with the one starting. --collect clears the
	// unit if it fails rather than leaving it behind to block the next one.
	runArgs := []string{"--user", "--collect", "--quiet"}
	for key, value := range setEnv {
		runArgs = append(runArgs, "--setenv="+key+"="+value)
	}
	for _, key := range forwardEnv {
		if value := os.Getenv(key); value != "" {
			runArgs = append(runArgs, "--setenv="+key+"="+value)
		}
	}
	runArgs = append(runArgs, "--", exe)
	runArgs = append(runArgs, args...)

	out, err := exec.Command(systemdRun, runArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemd-run: %w: %s", err, out)
	}
	return nil
}

// relaunchOutsideSteam hands the restart to the user's systemd manager.
func relaunchOutsideSteam(exe string, args []string) error {
	return systemdRunOutsideSteam(exe, args, map[string]string{brokeAwayEnv: "1"}, sessionEnvForRelaunch)
}
