package basic

import (
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/winutil"
)

func logFlow() *slog.Logger {
	return slog.Default().With("component", "basic-flow")
}

// logLoginFilesOrder emits the iteration order of LoginFiles for the given
// caller. Used to diagnose non-deterministic restore/save order across runs.
func logLoginFilesOrder(caller string, fc FlowContext) {
	keys := make([]string, 0, len(fc.Descriptor.LoginFiles))
	for k := range fc.Descriptor.LoginFiles {
		keys = append(keys, k)
	}
	sorted := append([]string(nil), keys...)
	sort.Strings(sorted)
	logFlow().Debug("LoginFiles iteration order", "caller", caller, "iterated", keys, "sorted", sorted)
}

func finishActionBarStatus(err *error) {
	if err != nil && *err != nil {
		platform.EmitActionBarStatusI18n("Status_FailedLog")
		return
	}
	platform.EmitActionBarStatus("")
}

func emitUpdatingFileStatus(path string) {
	file := strings.TrimSpace(filepath.Base(path))
	if file == "." || file == string(os.PathSeparator) {
		file = ""
	}
	if file == "" {
		file = strings.TrimSpace(path)
	}
	if file == "" {
		platform.EmitActionBarStatusI18n("Status_CopyingFiles")
		return
	}
	platform.EmitActionBarStatusI18nVars("Status_UpdatingFile", map[string]string{"file": file})
}

func wrapNeedsAdminIfPermission(err error) error {
	if err == nil || winutil.IsNeedsAdmin(err) {
		return err
	}
	if os.IsPermission(err) || strings.Contains(strings.ToLower(err.Error()), "access is denied") {
		return winutil.NewNeedsAdminError(err.Error())
	}
	return err
}
