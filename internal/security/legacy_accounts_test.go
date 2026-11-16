package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectPlainSavedSessionsIncludesLegacyAccounts(t *testing.T) {
	root := resetSecurityTest(t)
	base := filepath.Join(root, "LoginCache", "BattleNet")
	legacyDir := filepath.Join(base, "Player#1234")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(base, "ids.json")
	original := `{"player@example.test":"Player#1234"}`
	if err := os.WriteFile(index, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "email.json"), []byte(`"player@example.test"`), 0o600); err != nil {
		t.Fatal(err)
	}
	sessions, err := collectPlainSavedSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].PlainDir != legacyDir || sessions[0].UniqueID != "player@example.test" || sessions[0].AccountName != "Player#1234" {
		t.Fatalf("encryption enumeration missed the legacy session: %+v", sessions)
	}
	backup, err := os.ReadFile(index + ".legacy-dotnet.bak")
	if err != nil || string(backup) != original {
		t.Fatalf("legacy backup = %q, %v", backup, err)
	}
}
