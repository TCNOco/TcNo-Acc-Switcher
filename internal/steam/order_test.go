package steam

import (
	"testing"

	"TcNo-Acc-Switcher/internal/paths"
)

// The games strip offers the newest logins first only while the user has not
// arranged the list themselves, so "no order.json" has to answer false rather
// than fall back to some default order.
func TestHasSavedAccountOrder(t *testing.T) {
	paths.ResetForTest(t.TempDir())
	s := NewSteamService()

	saved, err := s.HasSavedAccountOrder()
	if err != nil {
		t.Fatal(err)
	}
	if saved {
		t.Fatal("HasSavedAccountOrder = true with no order.json written")
	}

	if err := SaveOrder([]string{"76561198000000201", "76561198000000202"}); err != nil {
		t.Fatal(err)
	}
	saved, err = s.HasSavedAccountOrder()
	if err != nil {
		t.Fatal(err)
	}
	if !saved {
		t.Fatal("HasSavedAccountOrder = false after SaveOrder")
	}
}
