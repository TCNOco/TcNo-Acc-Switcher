package steam

import (
	"strings"
	"testing"
)

func TestSetPersonaInFriendStoreJSON_EscapedSteamBlob(t *testing.T) {
	val := `{\\\"ePersonaState\\\":1,\\\"strNonFriendsAllowedToMsg\\\":\\\"\\\"}`
	out, ok := setPersonaInFriendStoreJSON(val, 7)
	if !ok {
		t.Fatal("expected patch")
	}
	if out == val {
		t.Fatal("expected change")
	}
	if strings.Count(out, "ePersonaState") != 1 {
		t.Fatalf("duplicate key fragment: %q", out)
	}
}

func TestSetPersonaInFriendStoreJSON_ValidJSON(t *testing.T) {
	val := `{"ePersonaState":1,"strNonFriendsAllowedToMsg":""}`
	out, ok := setPersonaInFriendStoreJSON(val, 7)
	if !ok || out == val {
		t.Fatalf("expected sjson path, got ok=%v out=%q", ok, out)
	}
}

func TestDedupeSerializedLocalConfigText(t *testing.T) {
	stacks := strings.Repeat(`\`, 12)
	s := `"trendingstore_storage"` + "\t\t\"" + "{" + stacks + `"version` + stacks + `":2}"`
	out := dedupeSerializedLocalConfigText(s)
	if strings.Contains(out, strings.Repeat(`\`, 6)) {
		end := 160
		if len(out) < end {
			end = len(out)
		}
		t.Fatalf("still stacked backslashes: %q", out[:end])
	}
}

func TestCollapseRepeatedBackslashes(t *testing.T) {
	// Simulates stacked backslashes inside a VDF string value (JSON blob).
	many := strings.Repeat(`\`, 16)
	in := `"FriendGroupCollapse_1"		"{` + many + `"groups` + many + `":{` + many + `"offline` + many + `":false`
	out := collapseRepeatedBackslashesStable(in)
	if strings.Contains(out, many+`"`) {
		t.Fatal("collapse should remove long backslash runs before quotes")
	}
	if !strings.Contains(out, `\"groups\"`) {
		end := 200
		if len(out) < end {
			end = len(out)
		}
		t.Fatalf("expected normalized \\\"groups\\\" fragment, got: %q", out[:end])
	}
}
