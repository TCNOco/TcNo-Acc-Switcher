package basic

import (
	"testing"
	"time"
)

func TestPruneExpiredTagsInFileRemovesExpiredAssignments(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	f := idsFile{
		IDs:      map[string]string{"u1": "One", "u2": "Two"},
		LastUsed: map[string]string{},
		Tags: map[string]tagFileEntry{
			"global-expired":  {Name: "Global", Color: "#111111", ExpiresAt: now.Add(-time.Hour).Format(time.RFC3339)},
			"account-expired": {Name: "Account", Color: "#222222"},
			"keep":            {Name: "Keep", Color: "#333333"},
		},
		AccountTags: map[string][]string{
			"u1": {"global-expired", "account-expired"},
			"u2": {"keep"},
		},
		AccountTagExpiries: map[string]map[string]string{
			"u1": {"account-expired": now.Add(-time.Minute).Format(time.RFC3339)},
			"u2": {"keep": now.Add(time.Hour).Format(time.RFC3339)},
		},
	}

	if !pruneExpiredTagsInFile(&f, now) {
		t.Fatalf("expected pruneExpiredTagsInFile to report changes")
	}
	if _, ok := f.AccountTags["u1"]; ok {
		t.Fatalf("u1 tags = %#v, want removed", f.AccountTags["u1"])
	}
	if got := f.AccountTags["u2"]; len(got) != 1 || got[0] != "keep" {
		t.Fatalf("u2 tags = %#v, want keep", got)
	}
	if len(f.Tags) != 1 {
		t.Fatalf("Tags = %#v, want only keep", f.Tags)
	}
	if _, ok := f.Tags["keep"]; !ok {
		t.Fatalf("keep tag missing after prune")
	}
	if got := f.AccountTagExpiries["u2"]["keep"]; got != now.Add(time.Hour).Format(time.RFC3339) {
		t.Fatalf("u2 keep expiry = %q", got)
	}
	if pruneExpiredTagsInFile(&f, now) {
		t.Fatalf("second prune should not report changes")
	}
}

func TestBuildAccountListContextPrunesExpiredTagsOnLoad(t *testing.T) {
	newFlowTestEnv(t)
	now := time.Now().UTC()
	if err := writeIdsFile("TestPlatform", idsFile{
		IDs:      map[string]string{"u1": "One"},
		LastUsed: map[string]string{},
		Tags: map[string]tagFileEntry{
			"expired": {Name: "Expired", Color: "#111111", ExpiresAt: now.Add(-time.Hour).Format(time.RFC3339)},
		},
		AccountTags: map[string][]string{
			"u1": {"expired"},
		},
	}); err != nil {
		t.Fatalf("write ids: %v", err)
	}

	svc := &BasicService{}
	ctx, err := svc.buildAccountListContext("TestPlatform")
	if err != nil {
		t.Fatalf("buildAccountListContext: %v", err)
	}
	if ctx == nil {
		t.Fatalf("buildAccountListContext returned nil context")
	}
	if got := resolveTagsForAccount(ctx.idf, "u1"); len(got) != 0 {
		t.Fatalf("resolved tags = %#v, want empty after prune", got)
	}

	f, err := readIdsFile("TestPlatform")
	if err != nil {
		t.Fatalf("read ids: %v", err)
	}
	if len(f.Tags) != 0 {
		t.Fatalf("Tags = %#v, want pruned on load", f.Tags)
	}
	if len(f.AccountTags) != 0 {
		t.Fatalf("AccountTags = %#v, want pruned on load", f.AccountTags)
	}
}

func TestSetTagExpiryStoresAccountAndGlobalExpiry(t *testing.T) {
	newFlowTestEnv(t)
	if err := writeIdsFile("TestPlatform", idsFile{
		IDs:      map[string]string{"u1": "One"},
		LastUsed: map[string]string{},
		Tags: map[string]tagFileEntry{
			"tag1": {Name: "Keep", Color: "#111111"},
		},
		AccountTags: map[string][]string{
			"u1": {"tag1"},
		},
	}); err != nil {
		t.Fatalf("write ids: %v", err)
	}

	svc := &BasicService{}
	accountExpiry := "2026-07-08T00:00:00+02:00"
	globalExpiry := "2026-07-09T00:00:00Z"
	if err := svc.SetTagExpiry("TestPlatform", "u1", "tag1", "account", accountExpiry); err != nil {
		t.Fatalf("SetTagExpiry account: %v", err)
	}
	if err := svc.SetTagExpiry("TestPlatform", "", "tag1", "all", globalExpiry); err != nil {
		t.Fatalf("SetTagExpiry all: %v", err)
	}

	f, err := readIdsFile("TestPlatform")
	if err != nil {
		t.Fatalf("read ids: %v", err)
	}
	if got := f.AccountTagExpiries["u1"]["tag1"]; got != "2026-07-07T22:00:00Z" {
		t.Fatalf("account expiry = %q", got)
	}
	if got := f.Tags["tag1"].ExpiresAt; got != globalExpiry {
		t.Fatalf("global expiry = %q", got)
	}
	if got := resolveTagsForAccount(f, "u1"); len(got) != 1 || got[0].ExpiresAt != "2026-07-07T22:00:00Z" {
		t.Fatalf("resolved tag expiry = %#v, want account expiry", got)
	}

	if err := svc.SetTagExpiry("TestPlatform", "u1", "tag1", "account", ""); err != nil {
		t.Fatalf("clear account expiry: %v", err)
	}
	if err := svc.SetTagExpiry("TestPlatform", "", "tag1", "all", ""); err != nil {
		t.Fatalf("clear global expiry: %v", err)
	}
	f, err = readIdsFile("TestPlatform")
	if err != nil {
		t.Fatalf("read ids after clear: %v", err)
	}
	if got := f.AccountTagExpiries["u1"]["tag1"]; got != "" {
		t.Fatalf("account expiry after clear = %q", got)
	}
	if got := f.Tags["tag1"].ExpiresAt; got != "" {
		t.Fatalf("global expiry after clear = %q", got)
	}
}

func TestRemoveTagFromAllAccountsPrunesDefinitionAndExpiries(t *testing.T) {
	newFlowTestEnv(t)
	if err := writeIdsFile("TestPlatform", idsFile{
		IDs:      map[string]string{"u1": "One", "u2": "Two"},
		LastUsed: map[string]string{},
		Tags: map[string]tagFileEntry{
			"tag1": {Name: "Keep", Color: "#111111"},
			"tag2": {Name: "Other", Color: "#222222"},
		},
		AccountTags: map[string][]string{
			"u1": {"tag1", "tag2"},
			"u2": {"tag1"},
		},
		AccountTagExpiries: map[string]map[string]string{
			"u2": {"tag1": "2026-07-09T00:00:00Z"},
		},
	}); err != nil {
		t.Fatalf("write ids: %v", err)
	}

	svc := &BasicService{}
	if err := svc.RemoveTagFromAllAccounts("TestPlatform", "tag1"); err != nil {
		t.Fatalf("RemoveTagFromAllAccounts: %v", err)
	}

	f, err := readIdsFile("TestPlatform")
	if err != nil {
		t.Fatalf("read ids: %v", err)
	}
	if _, ok := f.Tags["tag1"]; ok {
		t.Fatalf("tag1 definition should be pruned: %#v", f.Tags["tag1"])
	}
	if got := f.AccountTags["u1"]; len(got) != 1 || got[0] != "tag2" {
		t.Fatalf("u1 tags = %#v, want only tag2", got)
	}
	if _, ok := f.AccountTags["u2"]; ok {
		t.Fatalf("u2 tags = %#v, want removed", f.AccountTags["u2"])
	}
	if _, ok := f.AccountTagExpiries["u2"]; ok {
		t.Fatalf("u2 expiries = %#v, want removed", f.AccountTagExpiries["u2"])
	}
}

func TestApplySpecialTagCS2DropClaimedCreatesOrReusesTag(t *testing.T) {
	newFlowTestEnv(t)
	if err := writeIdsFile("TestPlatform", idsFile{
		IDs:      map[string]string{"u1": "One", "u2": "Two"},
		LastUsed: map[string]string{},
	}); err != nil {
		t.Fatalf("write ids: %v", err)
	}

	svc := &BasicService{}
	first, err := svc.ApplySpecialTag("TestPlatform", "u1", "cs2-drop-claimed")
	if err != nil {
		t.Fatalf("ApplySpecialTag first: %v", err)
	}
	second, err := svc.ApplySpecialTag("TestPlatform", "u2", "cs2-drop-claimed")
	if err != nil {
		t.Fatalf("ApplySpecialTag second: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("tag IDs differ: %q vs %q", first.ID, second.ID)
	}
	if first.Name != "CS2 Drop Claimed" {
		t.Fatalf("tag name = %q", first.Name)
	}

	f, err := readIdsFile("TestPlatform")
	if err != nil {
		t.Fatalf("read ids: %v", err)
	}
	if got := f.AccountTags["u1"]; len(got) != 1 || got[0] != first.ID {
		t.Fatalf("u1 tags = %#v", got)
	}
	if got := f.AccountTags["u2"]; len(got) != 1 || got[0] != first.ID {
		t.Fatalf("u2 tags = %#v", got)
	}
	expiry, err := time.Parse(time.RFC3339, f.Tags[first.ID].ExpiresAt)
	if err != nil {
		t.Fatalf("parse expiry: %v", err)
	}
	if expiry.Location() != time.UTC || expiry.Hour() != 1 || expiry.Minute() != 0 || expiry.Second() != 0 {
		t.Fatalf("expiry = %s, want 01:00:00Z", expiry.Format(time.RFC3339))
	}
	if expiry.Weekday() != time.Wednesday {
		t.Fatalf("expiry weekday = %s, want Wednesday", expiry.Weekday())
	}
}

func TestApplySpecialTagAllowsSteamIDAssignmentWithoutGenericIDEntry(t *testing.T) {
	newFlowTestEnv(t)
	if err := writeIdsFile("Steam", idsFile{
		IDs:      map[string]string{},
		LastUsed: map[string]string{},
	}); err != nil {
		t.Fatalf("write ids: %v", err)
	}

	const steamID64 = "76561199141170487"
	svc := &BasicService{}
	tag, err := svc.ApplySpecialTag("Steam", steamID64, "cs2-drop-claimed")
	if err != nil {
		t.Fatalf("ApplySpecialTag: %v", err)
	}

	f, err := readIdsFile("Steam")
	if err != nil {
		t.Fatalf("read ids: %v", err)
	}
	if got := f.AccountTags[steamID64]; len(got) != 1 || got[0] != tag.ID {
		t.Fatalf("steam account tags = %#v, want special tag assigned by SteamID64", got)
	}
}

func TestNextCS2DropReset(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "before reset this week",
			now:  time.Date(2026, 7, 7, 18, 0, 0, 0, time.UTC),
			want: time.Date(2026, 7, 8, 1, 0, 0, 0, time.UTC),
		},
		{
			name: "at reset rolls forward",
			now:  time.Date(2026, 7, 8, 1, 0, 0, 0, time.UTC),
			want: time.Date(2026, 7, 15, 1, 0, 0, 0, time.UTC),
		},
		{
			name: "after reset this week",
			now:  time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC),
			want: time.Date(2026, 7, 15, 1, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextCS2DropReset(tt.now); !got.Equal(tt.want) {
				t.Fatalf("nextCS2DropReset(%s) = %s, want %s", tt.now.Format(time.RFC3339), got.Format(time.RFC3339), tt.want.Format(time.RFC3339))
			}
		})
	}
}
