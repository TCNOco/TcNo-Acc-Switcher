package actionlog

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestSwitchAttemptsSurviveBackgroundActivity(t *testing.T) {
	Init()
	a := BeginSwitch("Steam", "target-id")
	a.Next("close_steam")
	a.Warning("close_steam", errors.New("access denied"))
	a.Next("launch_steam")
	a.Finish("", errors.New("executable not found"))
	for i := 0; i < maxLines+5; i++ {
		Record("file:write", "Statistics.json", "", nil)
	}
	text, ids := SwitchSnapshot()
	for _, want := range []string{"switch=1", "stage=close_steam", "outcome=warning", "stage=launch_steam outcome=fail", "executable not found", "result:failed", "login_verified:false"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if len(ids) != 1 || ids[0][0] != "target-id" {
		t.Fatalf("identifiers=%v", ids)
	}
	if strings.Contains(text, "Statistics.json") {
		t.Fatal("background writes crowded switch trace")
	}
}

func TestSwitchRetentionAndReset(t *testing.T) {
	Init()
	for i := 1; i <= maxSwitchAttempts+2; i++ {
		a := BeginSwitch("Steam", fmt.Sprintf("target-%d", i))
		a.Finish("settings_applied_launch_disabled", nil)
	}
	text, _ := SwitchSnapshot()
	if strings.Contains(text, "switch=1 ") || strings.Contains(text, "switch=2 ") {
		t.Fatal("old attempts retained")
	}
	if strings.Count(text, "stage=begin ") != maxSwitchAttempts {
		t.Fatal("incorrect retention")
	}
	Init()
	text, ids := SwitchSnapshot()
	if text != "" || len(ids) != 0 {
		t.Fatal("initialization retained previous session")
	}
}

func TestSwitchEventLimitKeepsContextAndOutcome(t *testing.T) {
	Init()
	a := BeginSwitch("Steam", "target")
	for i := 0; i < maxSwitchEvents+5; i++ {
		a.Note(map[string]any{"item": i})
	}
	a.Finish("launch_requested_login_unverified", nil)
	text, _ := SwitchSnapshot()
	if !strings.Contains(text, "stage=begin") || !strings.Contains(text, "stage=end") || !strings.Contains(text, "omitted_events=") {
		t.Fatal(text)
	}
	if len(a.events) > maxSwitchEvents {
		t.Fatal("unbounded trace")
	}
}

func TestSwitchRedactsCredentialsAndCopiesIdentifiers(t *testing.T) {
	Init()
	a := BeginSwitch("Steam", "76561198123456789")
	ids := []string{"alice", "76561198123456789"}
	a.Account(ids...)
	ids[0] = "changed"
	a.Next("launch")
	a.Note(map[string]any{"password": "PASSWORD_SENTINEL", "refresh_token": "TOKEN_SENTINEL"})
	a.Finish("failed", errors.New("access_token=ERROR_SENTINEL"))
	text, accounts := SwitchSnapshot()
	for _, secret := range []string{"PASSWORD_SENTINEL", "TOKEN_SENTINEL", "ERROR_SENTINEL"} {
		if strings.Contains(text, secret) {
			t.Fatalf("credential leaked: %s", text)
		}
	}
	if accounts[1][0] != "alice" {
		t.Fatal("caller changed stored identifiers")
	}
	accounts[1][0] = "changed snapshot"
	_, again := SwitchSnapshot()
	if again[1][0] != "alice" {
		t.Fatal("snapshot changed stored identifiers")
	}
}

func TestConcurrentSwitchTraces(t *testing.T) {
	Init()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a := BeginSwitch("Steam", "")
			a.Next("preflight")
			a.Note(map[string]any{"safe": true})
			_, _ = SwitchSnapshot()
			a.Finish("skipped_matching_account_markers", nil)
		}()
	}
	wg.Wait()
	text, _ := SwitchSnapshot()
	if strings.Count(text, "stage=end ") != maxSwitchAttempts {
		t.Fatal(text)
	}
}
