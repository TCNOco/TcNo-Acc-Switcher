package steam

import (
	"fmt"
	"strconv"
	"strings"

	"TcNo-Acc-Switcher/internal/actionlog"
)

func steamDiagnosticIDs(id, accountName, personaName string) []string {
	ids := []string{id, accountName, personaName}
	const steamIDBase = uint64(76561197960265728)
	if n, err := strconv.ParseUint(id, 10, 64); err == nil && n > steamIDBase && n-steamIDBase <= 0xffffffff {
		// Steam's userdata directory uses AccountID, not SteamID64.
		ids = append(ids, strconv.FormatUint(n-steamIDBase, 10))
	}
	return ids
}

// Record booleans about Steam's selection file, never session/token contents.
// These markers are written by us and cannot establish an authenticated login.
func recordSteamSwitchState(attempt *actionlog.SwitchAttempt, root, target string, verify bool) {
	if u, ok := knownAccountAsLoginUser(strings.TrimSpace(target)); ok {
		attempt.Account(steamDiagnosticIDs(u.SteamID64, u.AccountName, u.PersonaName)...)
	}
	users, err := ParseLoginUsers(LoginUsersPath(root))
	if err != nil {
		attempt.Warning("read_login_file", err)
		return
	}
	for _, u := range users {
		attempt.Account(steamDiagnosticIDs(u.SteamID64, u.AccountName, u.PersonaName)...)
	}

	details := steamSwitchSelectionDetails(users, target)
	attempt.Note(details)
	if verify && target != "" && details["selection_matches_target"] != true {
		attempt.Warning("verify_login_file", fmt.Errorf("written account selection does not match the requested target"))
	}
}

func steamSwitchSelectionDetails(users []LoginUser, target string) map[string]any {
	target = strings.TrimSpace(target)
	details := map[string]any{
		"account_count": len(users), "target_in_login_file": false,
		"target_has_login_name":    false,
		"selection_matches_target": target != "" && ActiveSessionSteamID64(users) == target,
	}
	for _, u := range users {
		if strings.TrimSpace(u.SteamID64) == target && target != "" {
			details["target_in_login_file"] = true
			details["target_has_login_name"] = strings.TrimSpace(u.AccountName) != ""
			details["remember_password_flag"] = u.RememberPassword == "1"
			details["allow_auto_login_flag"] = u.AutoLogin == "1"
			break
		}
	}
	return details
}
