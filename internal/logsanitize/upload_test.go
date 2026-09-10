package logsanitize

import (
	"errors"
	"strings"
	"testing"

	"TcNo-Acc-Switcher/internal/actionlog"
)

func TestSwitchUploadAliasesAccountsWithoutOnDiskAccountFiles(t *testing.T) {
	actionlog.Init()
	t.Cleanup(actionlog.Init)
	const id = "76561198679191951"
	const accountID = "718926223"
	const username = "PrivateLoginSentinel"
	const persona = "Private Persona Sentinel"
	a := actionlog.BeginSwitch("Steam", id)
	a.Account(id, accountID, username, persona)
	a.Next("launch_steam")
	a.Finish("failed", errors.New("launch failed for "+username+" refresh_token=SECRET_SENTINEL"))
	actionlog.Record("file:write", "D:/Steam/userdata/"+accountID+"/config/localconfig.vdf", "", nil)
	actionlog.Record("registry:write", "AutoLoginUser", username, nil)
	for i := 0; i < 600; i++ {
		actionlog.Record("file:write", "Statistics.json", "", nil)
	}
	text := ActionLogForUpload()
	for _, value := range []string{id, accountID, username, persona, "SECRET_SENTINEL"} {
		if strings.Contains(text, value) {
			t.Fatalf("private value %q leaked in %s", value, text)
		}
	}
	for _, want := range []string{"Recent switch attempts", "stage=launch_steam outcome=fail", "target:account1", "launch failed for account1", "userdata/account1/"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
}

func TestAccountGroupsShareOneAlias(t *testing.T) {
	accounts := [][]string{{"76561198679191951"}, {"718926223", "LoginSentinel"}, {"76561198679191951", "718926223", "PersonaSentinel"}}
	got := redactWithAccounts("76561198679191951 718926223 LoginSentinel PersonaSentinel", accounts)
	if got != "account1 account1 account1 account1" {
		t.Fatalf("aliases=%q", got)
	}
}

func TestAccountAliasesAreNotReplacedAgain(t *testing.T) {
	got := redactWithAccounts("76561198679191951 account1", [][]string{{"76561198679191951"}, {"account1"}})
	if got != "account1 account2" {
		t.Fatalf("aliases=%q", got)
	}
}

func TestAccountAliasesHandleUnicodeCaseFolding(t *testing.T) {
	got := redactWithAccounts("Kevin and kevin", [][]string{{"Kevin"}})
	if got != "account1 and account1" {
		t.Fatalf("aliases=%q", got)
	}
}
