package battlenetauth

import "testing"

func TestAccountNameCompatibilityAndDisplay(t *testing.T) {
	a := fixture()
	a.AccountName = "Example#1234"
	if err := a.Validate(); err != nil || a.DisplayName() != a.AccountName {
		t.Fatal("automatic name was not used")
	}
	raw, err := Export([]Account{a}, "fixture export password")
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := Import(raw, "fixture export password")
	if err != nil || len(accounts) != 1 || accounts[0].AccountName != a.AccountName {
		t.Fatal("account name was lost during export and import")
	}
	a.Label = "Manual name"
	if a.DisplayName() != a.Label {
		t.Fatal("automatic name replaced a manual name")
	}
	a.AccountName, a.Label = "", ""
	if a.Validate() != nil || a.DisplayName() != a.Serial {
		t.Fatal("older accounts without a captured name stopped working")
	}
}
