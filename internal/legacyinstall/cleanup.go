package legacyinstall

import (
	"fmt"
	"time"

	"TcNo-Acc-Switcher/internal/winutil"
)

// CleanupFlag runs a headless cleanup pass and exits. It is handled before the
// singleton mutex is taken, so the elevated copy this package spawns does not
// collide with the instance that spawned it.
const CleanupFlag = "--clean-legacy-install"

// elevatedTimeout bounds the wait on the elevated helper: a couple of hundred
// megabytes of small files on a slow disk, plus however long the user takes to
// answer the UAC prompt.
const elevatedTimeout = 5 * time.Minute

// CleanElevated removes the leftovers in dir, elevating first when the current
// process cannot write there. Returns what was removed. The user declining the
// UAC prompt surfaces as [winutil.ErrElevationDeclined].
func CleanElevated(dir string) (Result, error) {
	before := Detect(dir)
	if !before.Found() {
		return Result{}, nil
	}

	if Writable(dir) {
		res := Remove(before)
		pruneUninstallEntriesBestEffort(dir)
		return res, nil
	}

	if _, err := winutil.RunSelfElevatedAndWait([]string{CleanupFlag}, elevatedTimeout); err != nil {
		return Result{}, err
	}
	return removedBetween(before, Detect(dir)), nil
}

// RunCleanupCommand is the [CleanupFlag] entry point: scan, delete, report, exit.
// It assumes it is already elevated - main spawns it that way - and prints to the
// attached console so the flag is usable by hand.
func RunCleanupCommand(dir string) int {
	rep := Detect(dir)
	if !rep.Found() {
		fmt.Println("No files from the old version were found in", dir)
		return 0
	}
	fmt.Printf("Removing %d leftover entries (%s) from %s\n", rep.Count(), HumanBytes(rep.Bytes), dir)

	res := Remove(rep)
	for _, p := range res.Preserved {
		fmt.Println("  kept a copy:", p)
	}
	for _, f := range res.Failed {
		fmt.Printf("  failed: %s (%s)\n", f.Path, f.Error)
	}
	if keys, err := PruneUninstallEntries(dir); err != nil {
		fmt.Println("  failed to remove the old uninstall entry:", err)
	} else {
		for _, k := range keys {
			fmt.Println("  removed stale uninstall entry:", k)
		}
	}

	fmt.Printf("Removed %d entries, freed %s\n", len(res.Removed), HumanBytes(res.Bytes))
	if !res.Ok() {
		fmt.Printf("%d entries could not be removed; close any old TcNo processes and try again\n", len(res.Failed))
		return 1
	}
	return 0
}

// StartupCleanup handles leftovers found next to the running executable. A
// writable install directory (portable, or anywhere outside Program Files) is
// cleaned in the background; otherwise the work is left for the user to accept,
// since a UAC prompt fired unprompted at launch is not something to spring on
// anyone.
func StartupCleanup(dir string) {
	rep := Detect(dir)
	if !rep.Found() {
		return
	}
	if !Writable(dir) {
		removeLog().Info("found files from the old version; removal needs elevation",
			"dir", rep.ExeDir, "entries", rep.Count(), "size", HumanBytes(rep.Bytes),
			"dismissed", loadState().Dismissed)
		return
	}
	removeLog().Info("removing files from the old version",
		"dir", rep.ExeDir, "entries", rep.Count(), "size", HumanBytes(rep.Bytes))
	go func() {
		Remove(rep)
		pruneUninstallEntriesBestEffort(dir)
	}()
}

func pruneUninstallEntriesBestEffort(dir string) {
	keys, err := PruneUninstallEntries(dir)
	if err != nil {
		// Unelevated this is just access denied; the installer and the elevated
		// path both cover it.
		removeLog().Debug("could not remove the old uninstall entry", "err", err)
		return
	}
	for _, k := range keys {
		removeLog().Info("removed stale uninstall entry", "key", k)
	}
}

// removedBetween reports the entries present in before but gone in after, which
// is how the unelevated side learns what its elevated helper deleted.
func removedBetween(before, after Report) Result {
	remaining := make(map[string]struct{}, len(after.Entries))
	for _, e := range after.Entries {
		remaining[e.Path] = struct{}{}
	}
	var res Result
	for _, e := range before.Entries {
		if _, still := remaining[e.Path]; still {
			res.Failed = append(res.Failed, Failure{Path: e.Path, Error: "not removed"})
			continue
		}
		if e.Preserve {
			// Gone from the scan because it was renamed, not deleted, so it frees
			// no space and is not a removal.
			res.Preserved = append(res.Preserved, e.Path+BackupSuffix)
			continue
		}
		res.Removed = append(res.Removed, e.Path)
		res.Bytes += e.Bytes
	}
	return res
}

// HumanBytes formats a byte count for display, e.g. "248 MB".
func HumanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.0f %cB", float64(n)/float64(div), "KMGT"[exp])
}
