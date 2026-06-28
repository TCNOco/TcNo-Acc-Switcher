package basic

import "testing"

func TestAddTagToAccountsCreatesOrReusesTag(t *testing.T) {
	newFlowTestEnv(t)
	if err := writeIdsFile("TestPlatform", idsFile{
		IDs: map[string]string{
			"u1": "One",
			"u2": "Two",
			"u3": "Three",
		},
		LastUsed: map[string]string{},
		Tags: map[string]tagFileEntry{
			"existing": {Name: "Keep", Color: "#112233"},
		},
		AccountTags: map[string][]string{
			"u1": {"existing"},
		},
	}); err != nil {
		t.Fatalf("write ids: %v", err)
	}

	svc := &BasicService{}
	tag, err := svc.AddTagToAccounts("TestPlatform", []string{"u1", "u2", "missing", "u2"}, "Keep")
	if err != nil {
		t.Fatalf("AddTagToAccounts: %v", err)
	}
	if tag.ID != "existing" {
		t.Fatalf("tag ID = %q, want existing", tag.ID)
	}
	f, err := readIdsFile("TestPlatform")
	if err != nil {
		t.Fatalf("read ids: %v", err)
	}
	if got := f.AccountTags["u1"]; len(got) != 1 || got[0] != "existing" {
		t.Fatalf("u1 tags = %#v, want existing once", got)
	}
	if got := f.AccountTags["u2"]; len(got) != 1 || got[0] != "existing" {
		t.Fatalf("u2 tags = %#v, want existing", got)
	}
	if _, ok := f.AccountTags["missing"]; ok {
		t.Fatalf("missing account got tags: %#v", f.AccountTags["missing"])
	}
}

func TestClearAccountTagsPrunesDefinitions(t *testing.T) {
	newFlowTestEnv(t)
	if err := writeIdsFile("TestPlatform", idsFile{
		IDs:      map[string]string{"u1": "One", "u2": "Two"},
		LastUsed: map[string]string{},
		Tags: map[string]tagFileEntry{
			"a": {Name: "A", Color: "#111111"},
			"b": {Name: "B", Color: "#222222"},
		},
		AccountTags: map[string][]string{
			"u1": {"a"},
			"u2": {"b"},
		},
	}); err != nil {
		t.Fatalf("write ids: %v", err)
	}

	svc := &BasicService{}
	if err := svc.ClearAccountTags("TestPlatform"); err != nil {
		t.Fatalf("ClearAccountTags: %v", err)
	}
	f, err := readIdsFile("TestPlatform")
	if err != nil {
		t.Fatalf("read ids: %v", err)
	}
	if len(f.AccountTags) != 0 {
		t.Fatalf("AccountTags = %#v, want empty", f.AccountTags)
	}
	if len(f.Tags) != 0 {
		t.Fatalf("Tags = %#v, want pruned", f.Tags)
	}
}
