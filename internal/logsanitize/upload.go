package logsanitize

import "TcNo-Acc-Switcher/internal/actionlog"

// ActionLogForUpload returns the pruned session action log with account identifiers redacted.
func ActionLogForUpload() string {
	return Redact(actionlog.SnapshotPruned(actionlog.DefaultPruneFirst, actionlog.DefaultPruneLast))
}
