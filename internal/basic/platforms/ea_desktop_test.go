package platforms

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"TcNo-Acc-Switcher/internal/platform"
)

func TestEACacheLayouts(t *testing.T) {
	root := filepath.Join("testdata", "ea_desktop")
	legacy := filepath.Join(root, "CEF", "BrowserCache", "EADesktop", "Cache", "Cache_Data", "data_*")
	modern := filepath.Join(root, "CEF", "*", "EADesktop", "Default", "Cache", "Cache_Data", "data_*")
	for _, pattern := range []string{legacy, modern} {
		t.Run(pattern, func(t *testing.T) {
			files, err := cacheDataFiles(pattern, legacy, modern)
			if err != nil || len(files) != 2 {
				t.Fatalf("cache discovery = %v, %v; want both layouts without duplicates", files, err)
			}
			userPattern := filepath.Join(root, "user_*.ini")
			name, err := EASuggestedName(pattern, userPattern, legacy, modern)
			if err != nil || name != "CurrentPlayer" {
				t.Fatalf("name = %q, %v; want current user's display name", name, err)
			}
			image, err := EAImageSource(pattern, userPattern, legacy, modern)
			if err != nil || image.RemoteURL != "https://example.com/current.png" {
				t.Fatalf("image = %+v, %v; want current user's avatar", image, err)
			}
		})
	}
	// No implicit sibling discovery, even for a path inside CEF.
	files, err := cacheDataFiles(legacy)
	if err != nil || len(files) != 1 {
		t.Fatalf("custom pattern = %v, %v", files, err)
	}
	if _, err := cacheDataFiles("["); err == nil {
		t.Fatal("invalid glob should return an error")
	}
}

func TestEAShippedCachePatterns(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "Platforms.json"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := platform.ParseDescriptor(raw, "EA Desktop")
	if err != nil {
		t.Fatal(err)
	}
	// Expand the shipped descriptor against the read-only fixture tree.
	root, err := filepath.Abs(filepath.Join("testdata", "ea_desktop"))
	if err != nil {
		t.Fatal(err)
	}
	const prefix = `%LocalAppData%\Electronic Arts\EA Desktop`
	expand := func(value string) string {
		return filepath.FromSlash(strings.ReplaceAll(strings.ReplaceAll(value, prefix, root), `\`, "/"))
	}
	users := expand(d.Extras.BuiltInUserId)
	if len(d.Extras.BuiltInUsernameFileFallbacks) != 1 || len(d.Extras.BuiltInProfileImageFileFallbacks) != 1 {
		t.Fatal("EA catalog must include the legacy cache layout for names and avatars")
	}
	name, err := EASuggestedName(expand(d.Extras.BuiltInUsernameFile), users, expand(d.Extras.BuiltInUsernameFileFallbacks[0]))
	if err != nil || name != "CurrentPlayer" {
		t.Fatalf("catalog name = %q, %v", name, err)
	}
	image, err := EAImageSource(expand(d.Extras.BuiltInProfileImageFile), users, expand(d.Extras.BuiltInProfileImageFileFallbacks[0]))
	if err != nil || image.RemoteURL != "https://example.com/current.png" {
		t.Fatalf("catalog image = %+v, %v", image, err)
	}
}

func TestEANameMatchesAccount(t *testing.T) {
	data := []byte("\x00" + `{"data":{"me":{"id":"12345","player":{"displayName":"CommunityName","nickname":"OldNickname"}}}}` + "\x00")
	if got := parseEANameForUserIDs(data, []string{"12345"}); got != "CommunityName" {
		t.Fatalf("name = %q; want display name", got)
	}
	if got := parseEANameForUserIDs(data, []string{"67890"}); got != "" {
		t.Fatalf("returned another account's name: %q", got)
	}
}
