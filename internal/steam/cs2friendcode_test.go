package steam

import "testing"

// Rabscuttle's account, the vector both independent reverse-engineered
// implementations pin themselves to (emily33901/js-csfriendcode,
// not-wlan/csgo-friendcode). Nothing about the encoding is derivable, so a
// wrong shuffle only shows up against a known pair.
func TestCS2FriendCode(t *testing.T) {
	cases := []struct {
		id64 string
		acc  uint32
		want string
	}{
		{"76561197960287930", 22202, "SUCVS-FADA"},
	}

	for _, c := range cases {
		if got := CS2FriendCode(c.acc); got != c.want {
			t.Errorf("CS2FriendCode(%d) = %q, want %q", c.acc, got, c.want)
		}

		f, err := FormatsFromID64(c.id64)
		if err != nil {
			t.Fatalf("FormatsFromID64(%s): %v", c.id64, err)
		}
		if f.CS2FriendCode != c.want {
			t.Errorf("FormatsFromID64(%s).CS2FriendCode = %q, want %q", c.id64, f.CS2FriendCode, c.want)
		}
		// The Steam friend code Steam's own "Add a Friend" page shows is the
		// account ID, so it must stay in step with SteamID32.
		if f.FriendCode != f.ID32 {
			t.Errorf("FormatsFromID64(%s): FriendCode %q != ID32 %q", c.id64, f.FriendCode, f.ID32)
		}
	}
}
