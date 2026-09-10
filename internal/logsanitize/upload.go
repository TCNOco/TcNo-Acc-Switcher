package logsanitize

import "TcNo-Acc-Switcher/internal/actionlog"

// ActionLogForUpload preserves recent switch attempts separately from noisy
// background file operations. Both sections use the same account aliases.
func ActionLogForUpload() string {
	attempts, accounts := actionlog.SwitchSnapshot()
	text := actionlog.SnapshotPruned(actionlog.DefaultPruneFirst, actionlog.DefaultPruneLast)
	if attempts != "" {
		text = "Recent switch attempts (login acceptance is not verified):\n" + attempts + "\n\nSession file/registry actions:\n" + text
	}
	accounts = append(accounts, collectAccountIdentifiers()...)
	return redactWithAccounts(text, accounts)
}
