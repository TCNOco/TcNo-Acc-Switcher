package basic

import (
	"os"
	"path/filepath"
	"testing"

	"TcNo-Acc-Switcher/internal/platform"
)

func TestDescriptorCachedJSONName(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "Platforms.json"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := platform.ParseDescriptor(raw, "Ubisoft")
	if err != nil {
		t.Fatal(err)
	}
	if d.Extras.BuiltInUsernameFile == "" || len(d.Extras.BuiltInUsernameFileFallbacks) == 0 || d.Extras.BuiltInUsernameJSONPath != "nameOnPlatform" || d.Extras.BuiltInUsernameJSONMatchPath != "userId" {
		t.Fatal("Ubisoft descriptor must select the current account's community name")
	}
	// Exercise the shared descriptor resolver against anonymous, read-only data.
	root, err := filepath.Abs(filepath.Join("platforms", "testdata", "cache_providers"))
	if err != nil {
		t.Fatal(err)
	}
	d.Extras.BuiltInUsernameFile = `%Platform_Folder%\geforce_now.txt`
	d.Extras.BuiltInUsernameFileFallbacks = []string{`%Platform_Folder%\json_profile.txt`}
	name, err := descriptorCachedJSONSaveName(d, root, platform.PathTokenContext{}, nil, "current-user")
	if err != nil || name != "CurrentPlayer" {
		t.Fatalf("resolved name = %q, %v", name, err)
	}
	if name, err := descriptorCachedJSONSaveName(d, root, platform.PathTokenContext{}, nil, ""); err == nil || name != "" {
		t.Fatalf("missing current account accepted: %q, %v", name, err)
	}
}
