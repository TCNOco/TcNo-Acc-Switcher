package steam

import "testing"

func TestBuildSteamAccountListItemsProjectsGuardStateWithoutReordering(t *testing.T) {
	users := []LoginUser{
		{SteamID64: "76561198000000001", AccountName: " first ", PersonaName: "One"},
		{SteamID64: "76561198000000002", AccountName: "second", PersonaName: "Two"},
	}
	states := map[string]SteamGuardAccountState{
		users[0].SteamID64: {HasSteamGuard: true},
		users[1].SteamID64: {Pending: true},
	}

	got := buildSteamAccountListItems(users, users[1].SteamID64, states)
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2", len(got))
	}
	if got[0].SteamID64 != users[0].SteamID64 || got[1].SteamID64 != users[1].SteamID64 {
		t.Fatalf("account ordering changed: %#v", got)
	}
	if !got[0].HasSteamGuard || got[0].SteamGuardPending {
		t.Fatalf("unexpected first Steam Guard state: %#v", got[0])
	}
	if got[1].HasSteamGuard || !got[1].SteamGuardPending || !got[1].CurrentSession {
		t.Fatalf("unexpected second Steam Guard/session state: %#v", got[1])
	}
	if got[0].AccountName != "first" {
		t.Fatalf("account name was not trimmed: %q", got[0].AccountName)
	}
}
