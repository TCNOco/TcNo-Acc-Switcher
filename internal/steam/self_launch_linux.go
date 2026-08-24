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

// relaunchOutsideSteam hands the restart to the user's systemd manager, which is
// the one process that can start the app somewhere Steam's reaper cannot see it.
// Forking is no use: the reaper is a subreaper, so an orphan reparents onto it
// rather than to init, and dies with Steam anyway.
func relaunchOutsideSteam(exe string, args []string) error {
	systemdRun, err := exec.LookPath("systemd-run")
	if err != nil {
		return fmt.Errorf("systemd-run is needed to start outside Steam's process tree: %w", err)
	}

	// No --unit: letting systemd name the transient unit means a switcher that is
	// already running cannot collide with the one starting. --collect clears the
	// unit if it fails rather than leaving it behind to block the next one.
	runArgs := []string{"--user", "--collect", "--quiet", "--setenv=" + brokeAwayEnv + "=1"}
	for _, key := range sessionEnvForRelaunch {
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
