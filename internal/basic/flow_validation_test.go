package basic

import (
	"errors"
	"os"
	"testing"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/winutil"
)

func TestSavedAccountNameCannotReplaceAnotherIdentity(t *testing.T) {
	ids := map[string]string{"account-a": "Player One", "account-b": "Other"}
	for _, name := range []string{"Player One", "player one"} {
		if err := validateAccountSaveName(ids, "account-b", name); err == nil {
			t.Fatalf("cross-account replacement allowed for %q", name)
		}
	}
	if err := validateAccountSaveName(ids, "ACCOUNT-A", "Player One"); err != nil {
		t.Fatal(err)
	}
	if err := validateAccountSaveName(ids, "account-c", "New Player"); err != nil {
		t.Fatal(err)
	}
	if err := validateAccountSaveName(map[string]string{"a": "Player?One"}, "b", "Player_One"); err == nil {
		t.Fatal("normalized filename collision allowed")
	}
	if len(ids) != 2 || ids["account-a"] != "Player One" {
		t.Fatal("rejected replacement modified the account index")
	}
}

func TestCredentialSessionRetryDoesNotRewindRefreshedAccount(t *testing.T) {
	fc := FlowContext{Descriptor: platform.Descriptor{Extras: platform.DescriptorExtras{CredentialSession: &platform.CredentialSessionDescriptor{}}}}
	for _, err := range []error{nil, os.ErrNotExist, winutil.ErrCredentialNotFound} {
		if !shouldRestoreSavedSession(fc, "", "account-a", err) {
			t.Fatalf("signed-out session blocked restoring a saved account: %v", err)
		}
	}
	if !shouldRestoreSavedSession(fc, "", "account-a", errors.New("identity file temporarily locked")) {
		t.Fatal("unreadable identity did not require target preparation")
	}
	if shouldRestoreSavedSession(fc, "ACCOUNT-A", "account-a", nil) {
		t.Fatal("retry identifying the active account would reinstall a stale snapshot")
	}
	if !shouldRestoreSavedSession(fc, "account-b", "account-a", nil) {
		t.Fatal("different active account would skip restoration")
	}
	if !shouldRestoreSavedSession(FlowContext{}, "account-a", "account-a", nil) {
		t.Fatal("credential-specific shortcut changed ordinary platform restoration")
	}
}
