package basic

import (
	"path/filepath"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
)

const managedTagPlatform = "Steam"
const managedTagAccount = "76561198000000001"

func useManagedTagRoot(t *testing.T) {
	t.Helper()
	exeDir := t.TempDir()
	platform.ResetPathSingletonsForTest(exeDir)
	paths.ResetForTest(filepath.Join(exeDir, "TcNo Account Switcher"))
}

func accountTagNames(t *testing.T, uniqueID string) []string {
	t.Helper()
	f, err := readIdsFile(managedTagPlatform)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(f.AccountTags[uniqueID]))
	for _, id := range f.AccountTags[uniqueID] {
		names = append(names, f.Tags[id].Name)
	}
	return names
}

func accountTagExpiry(t *testing.T, uniqueID, name string) string {
	t.Helper()
	f, err := readIdsFile(managedTagPlatform)
	if err != nil {
		t.Fatal(err)
	}
	for id, def := range f.Tags {
		if def.Name == name {
			return f.AccountTagExpiries[uniqueID][id]
		}
	}
	return ""
}

func TestSetManagedTagCreatesTheDefinitionAndAssignsIt(t *testing.T) {
	useManagedTagRoot(t)
	expiry := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)

	if err := SetManagedTag(managedTagPlatform, managedTagAccount, ManagedTagCS2Cooldown, true, expiry.Format(time.RFC3339)); err != nil {
		t.Fatalf("SetManagedTag: %v", err)
	}
	if got := accountTagNames(t, managedTagAccount); len(got) != 1 || got[0] != ManagedTagCS2Cooldown {
		t.Fatalf("tags = %#v", got)
	}
	if got := accountTagExpiry(t, managedTagAccount, ManagedTagCS2Cooldown); got != expiry.Format(time.RFC3339) {
		t.Fatalf("account expiry = %q, want %q", got, expiry.Format(time.RFC3339))
	}
}

func TestSetManagedTagNeverWritesADefinitionLevelExpiry(t *testing.T) {
	// The single most important property. pruneExpiredTagsInFile strips a
	// definition-expired tag from EVERY account, so one account's cooldown
	// lapsing would untag everyone else still on cooldown.
	useManagedTagRoot(t)
	expiry := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	if err := SetManagedTag(managedTagPlatform, managedTagAccount, ManagedTagCS2Cooldown, true, expiry); err != nil {
		t.Fatal(err)
	}
	f, err := readIdsFile(managedTagPlatform)
	if err != nil {
		t.Fatal(err)
	}
	for id, def := range f.Tags {
		if def.ExpiresAt != "" {
			t.Fatalf("definition %s has ExpiresAt %q, want empty", id, def.ExpiresAt)
		}
	}
}

func TestSetManagedTagRewritesTheExpiryOnReapply(t *testing.T) {
	// Every add path calls clearAccountTagExpiry, so an apply that only set the
	// expiry on first assignment would leave the tag permanent from then on.
	useManagedTagRoot(t)
	first := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	second := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Second)

	if err := SetManagedTag(managedTagPlatform, managedTagAccount, ManagedTagCS2Cooldown, true, first.Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	if err := SetManagedTag(managedTagPlatform, managedTagAccount, ManagedTagCS2Cooldown, true, second.Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	if got := accountTagExpiry(t, managedTagAccount, ManagedTagCS2Cooldown); got != second.Format(time.RFC3339) {
		t.Fatalf("expiry = %q, want the reapplied %q", got, second.Format(time.RFC3339))
	}
	if got := accountTagNames(t, managedTagAccount); len(got) != 1 {
		t.Fatalf("reapply duplicated the assignment: %#v", got)
	}
}

func TestSetManagedTagWithoutAnExpiryLeavesItUnset(t *testing.T) {
	// A permanent cooldown must not be pruned away.
	useManagedTagRoot(t)
	if err := SetManagedTag(managedTagPlatform, managedTagAccount, ManagedTagCS2Cooldown, true, ""); err != nil {
		t.Fatal(err)
	}
	if got := accountTagExpiry(t, managedTagAccount, ManagedTagCS2Cooldown); got != "" {
		t.Fatalf("expiry = %q, want empty", got)
	}
	f, _ := readIdsFile(managedTagPlatform)
	if pruneExpiredTagsInFile(&f, time.Now().Add(100*365*24*time.Hour)) {
		t.Fatal("an expiry-less managed tag was pruned")
	}
}

func TestSetManagedTagRemovesTheAssignment(t *testing.T) {
	useManagedTagRoot(t)
	if err := SetManagedTag(managedTagPlatform, managedTagAccount, ManagedTagCS2Cooldown, true, ""); err != nil {
		t.Fatal(err)
	}
	if err := SetManagedTag(managedTagPlatform, managedTagAccount, ManagedTagCS2Cooldown, false, ""); err != nil {
		t.Fatal(err)
	}
	if got := accountTagNames(t, managedTagAccount); len(got) != 0 {
		t.Fatalf("tags = %#v, want none", got)
	}
	// The definition goes with it, since no account holds it any more.
	f, _ := readIdsFile(managedTagPlatform)
	if len(f.Tags) != 0 {
		t.Fatalf("Tags = %#v, want the unused definition pruned", f.Tags)
	}
}

func TestSetManagedTagLeavesOtherAccountsAlone(t *testing.T) {
	useManagedTagRoot(t)
	const other = "76561198000000002"
	for _, id := range []string{managedTagAccount, other} {
		if err := SetManagedTag(managedTagPlatform, id, ManagedTagCS2Cooldown, true, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := SetManagedTag(managedTagPlatform, managedTagAccount, ManagedTagCS2Cooldown, false, ""); err != nil {
		t.Fatal(err)
	}
	if got := accountTagNames(t, managedTagAccount); len(got) != 0 {
		t.Fatalf("target tags = %#v, want none", got)
	}
	if got := accountTagNames(t, other); len(got) != 1 {
		t.Fatalf("other account tags = %#v, want the tag kept", got)
	}
}

func TestSetManagedTagRemovalIsIdempotent(t *testing.T) {
	useManagedTagRoot(t)
	if err := SetManagedTag(managedTagPlatform, managedTagAccount, ManagedTagCS2Cooldown, false, ""); err != nil {
		t.Fatalf("removing an absent tag: %v", err)
	}
}

func TestSetManagedTagKeepsUnrelatedTags(t *testing.T) {
	useManagedTagRoot(t)
	if err := SetManagedTag(managedTagPlatform, managedTagAccount, "Main", true, ""); err != nil {
		t.Fatal(err)
	}
	if err := SetManagedTag(managedTagPlatform, managedTagAccount, ManagedTagCS2Cooldown, true, ""); err != nil {
		t.Fatal(err)
	}
	if err := SetManagedTag(managedTagPlatform, managedTagAccount, ManagedTagCS2Cooldown, false, ""); err != nil {
		t.Fatal(err)
	}
	if got := accountTagNames(t, managedTagAccount); len(got) != 1 || got[0] != "Main" {
		t.Fatalf("tags = %#v, want only Main", got)
	}
}

func TestSetManagedTagRejectsAMalformedExpiry(t *testing.T) {
	useManagedTagRoot(t)
	if err := SetManagedTag(managedTagPlatform, managedTagAccount, ManagedTagCS2Cooldown, true, "not-a-time"); err == nil {
		t.Fatal("SetManagedTag accepted a malformed expiry")
	}
	if got := accountTagNames(t, managedTagAccount); len(got) != 0 {
		t.Fatalf("tags = %#v, want none written", got)
	}
}

func TestClearManagedTagRemovesItFromEveryAccount(t *testing.T) {
	// The switch that turns a managed tag off has to undo it everywhere: it
	// cannot be removed by hand, so anything left behind would be stuck.
	useManagedTagRoot(t)
	const other = "76561198000000002"
	for _, uid := range []string{managedTagAccount, other} {
		if err := SetManagedTag(managedTagPlatform, uid, ManagedTagCS2Prime, true, ""); err != nil {
			t.Fatal(err)
		}
	}
	// A tag the user owns must survive the sweep-wide clear.
	if err := SetManagedTag(managedTagPlatform, managedTagAccount, "Trade", true, ""); err != nil {
		t.Fatal(err)
	}

	if err := ClearManagedTag(managedTagPlatform, ManagedTagCS2Prime); err != nil {
		t.Fatalf("ClearManagedTag: %v", err)
	}
	if got := accountTagNames(t, managedTagAccount); len(got) != 1 || got[0] != "Trade" {
		t.Fatalf("tags = %v, want only the user's own tag", got)
	}
	if got := accountTagNames(t, other); len(got) != 0 {
		t.Fatalf("tags = %v, want none", got)
	}
	f, err := readIdsFile(managedTagPlatform)
	if err != nil {
		t.Fatal(err)
	}
	for _, def := range f.Tags {
		if def.Name == ManagedTagCS2Prime {
			t.Fatal("definition survived with no account holding it")
		}
	}
}

func TestClearManagedTagIsANoOpWhenAbsent(t *testing.T) {
	useManagedTagRoot(t)
	if err := ClearManagedTag(managedTagPlatform, ManagedTagCS2NonPrime); err != nil {
		t.Fatalf("ClearManagedTag on an untagged platform: %v", err)
	}
}

func TestManagedTagsUseDistinctFixedColours(t *testing.T) {
	// pruneUnusedTagDefinitions deletes a definition the moment its last account
	// loses it, so these are created and destroyed repeatedly; a colour that
	// varied per creation would look broken.
	seen := map[string]string{}
	for _, name := range ManagedTagNames() {
		color := managedTagColorFor(name)
		if color == "" {
			t.Fatalf("%s has no colour", name)
		}
		if prev, dup := seen[color]; dup {
			t.Fatalf("%s and %s share colour %s", prev, name, color)
		}
		seen[color] = name
	}
}
