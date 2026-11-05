package steamguard

import (
	"encoding/json"
	"strings"
	"testing"

	"TcNo-Acc-Switcher/internal/battlenetauth"
)

func TestBattleNetBackupSummaryDoesNotExposeSecrets(t *testing.T) {
	a := battlenetauth.Account{Version: 1, Serial: "US-1234-5678-9012", Label: "Example", DeviceSecret: "3132333435363738393031323334353637383930", RestoreCode: "0123456789"}
	raw, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := describeBackupRecord(raw)
	if err != nil {
		t.Fatal(err)
	}
	if summary.accountName != "Battle.net: Example" || summary.tokenExpiry != 0 || strings.Contains(summary.accountName, a.DeviceSecret) || strings.Contains(summary.accountName, a.RestoreCode) {
		t.Fatal("incorrect or sensitive backup summary")
	}
	if err := validateBattleNetBackupIdentity("battlenet:"+a.Serial, raw); err != nil {
		t.Fatal(err)
	}
	for _, identity := range []string{"76561198000000000", "battlenet:EU-1234-5678-9012"} {
		if err := validateBattleNetBackupIdentity(identity, raw); err == nil {
			t.Fatal("mismatched backup identity accepted")
		}
	}
}
