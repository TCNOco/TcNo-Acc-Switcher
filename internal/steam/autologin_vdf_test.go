//go:build !windows

package steam

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestRegistryVDFPathDerivesFromRoot pins where the AutoLoginUser write lands.
// Linux keeps registry.vdf a level out from the install root, so a Flatpak Steam
// and a native one have to resolve to their own copies - writing the native
// file for a Flatpak switch would set an account the Flatpak client never reads.
func TestRegistryVDFPathDerivesFromRoot(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("macOS keeps registry.vdf inside the install root")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)

	cases := []struct {
		name string
		root string
		want string
	}{
		{
			name: "native",
			root: filepath.Join(home, ".local", "share", "Steam"),
			want: filepath.Join(home, ".steam", "registry.vdf"),
		},
		{
			name: "flatpak",
			root: filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
			want: filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".steam", "registry.vdf"),
		},
		{
			// Steam creates ~/.steam wherever the data lives, so a root that is
			// not home-shaped still reads its login from the one in $HOME.
			name: "root elsewhere falls back to $HOME",
			root: filepath.Join(home, "games", "SteamLibrary"),
			want: filepath.Join(home, ".steam", "registry.vdf"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := registryVDFPath(tc.root); got != tc.want {
				t.Errorf("registryVDFPath(%q) = %q, want %q", tc.root, got, tc.want)
			}
		})
	}
}

// TestWriteAutoLoginKeepsSteamsOtherKeys is the invariant that matters: the file
// is Steam's, and it holds language, the client launcher type and whatever else
// Steam decided to keep there. Rewriting it as if it only contained the login
// would silently reset all of it on every switch.
func TestWriteAutoLoginKeepsSteamsOtherKeys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".local", "share", "Steam")

	path := registryVDFPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `"Registry"
{
	"HKLM"
	{
		"Software"
		{
			"Valve"
			{
				"Steam"
				{
					"SteamPID"		"1234"
				}
			}
		}
	}
	"HKCU"
	{
		"Software"
		{
			"Valve"
			{
				"Steam"
				{
					"language"		"english"
					"AutoLoginUser"		"previous"
					"AlreadyRetriedOfflineMode"		"0"
				}
			}
		}
	}
}
`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeAutoLogin(root, "alpha"); err != nil {
		t.Fatalf("writeAutoLogin: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := string(got)

	for _, want := range []string{
		`"AutoLoginUser"`, `"alpha"`,
		`"RememberPassword"`, `"1"`,
		`"language"`, `"english"`,
		`"AlreadyRetriedOfflineMode"`,
		`"SteamPID"`, `"1234"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("registry.vdf lost %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, `"previous"`) {
		t.Errorf("registry.vdf still names the old account:\n%s", out)
	}
}

// TestWriteAutoLoginCreatesMissingFile covers the account that has never signed
// in on this machine: Steam has not written a registry.vdf yet, and the switch
// still has to leave one naming the account.
func TestWriteAutoLoginCreatesMissingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".local", "share", "Steam")

	if err := writeAutoLogin(root, "beta"); err != nil {
		t.Fatalf("writeAutoLogin: %v", err)
	}
	got, err := os.ReadFile(registryVDFPath(root))
	if err != nil {
		t.Fatalf("registry.vdf was not created: %v", err)
	}
	for _, want := range []string{`"HKCU"`, `"Valve"`, `"AutoLoginUser"`, `"beta"`} {
		if !strings.Contains(string(got), want) {
			t.Errorf("created registry.vdf missing %s:\n%s", want, got)
		}
	}
}

// TestWriteAutoLoginClearsForAddNew covers "Add New": no account is selected, so
// Steam must be left on its own account chooser rather than signed in as
// whoever was there before.
func TestWriteAutoLoginClearsForAddNew(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".local", "share", "Steam")

	if err := writeAutoLogin(root, "alpha"); err != nil {
		t.Fatalf("writeAutoLogin: %v", err)
	}
	if err := writeAutoLogin(root, ""); err != nil {
		t.Fatalf("writeAutoLogin clear: %v", err)
	}
	got, err := os.ReadFile(registryVDFPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), `"alpha"`) {
		t.Errorf("registry.vdf still names an account after Add New:\n%s", got)
	}
}
