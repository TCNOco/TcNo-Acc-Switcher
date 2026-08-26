//go:build !windows

package steam

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A Flatpak Steam and a native one must resolve to their own registry.vdf:
// writing the native file for a Flatpak switch sets an account it never reads.
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
			// The snap runs Valve's own launcher with HOME=$SNAP_USER_COMMON, so
			// the whole native layout repeats one level down.
			name: "snap",
			root: filepath.Join(home, "snap", "steam", "common", ".local", "share", "Steam"),
			want: filepath.Join(home, "snap", "steam", "common", ".steam", "registry.vdf"),
		},
		{
			// Debian's steam-installer puts the client in ~/.steam itself, so the
			// tail never matches and the $HOME fallback is the right answer.
			name: "debian-installation",
			root: filepath.Join(home, ".steam", "debian-installation"),
			want: filepath.Join(home, ".steam", "registry.vdf"),
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

// The file is Steam's and holds far more than the login. Rewriting it as if it
// did not would reset the rest on every switch.
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

// Steam has not written a registry.vdf until it first signs in.
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

// Add New selects no account, so Steam must land on its own chooser.
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

// Read and write must be inverse, or Steam's escaped paths gain another layer of
// backslashes on every switch.
func TestWriteAutoLoginDoesNotStackEscapes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".local", "share", "Steam")
	path := registryVDFPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	// One logical backslash, written the way Steam writes it.
	const escapedPath = `/home/u/.local/share/Steam/steamapps\\sourcemods`
	existing := "\"Registry\"\n{\n\t\"HKCU\"\n\t{\n\t\t\"Software\"\n\t\t{\n\t\t\t\"Valve\"\n\t\t\t{\n\t\t\t\t\"Steam\"\n\t\t\t\t{\n\t\t\t\t\t\"SourceModInstallPath\"\t\t\"" + escapedPath + "\"\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n}\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	for i := 1; i <= 3; i++ {
		if err := writeAutoLogin(root, "alpha"); err != nil {
			t.Fatalf("writeAutoLogin #%d: %v", i, err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), escapedPath) {
			t.Fatalf("after switch #%d the escaping changed:\n%s", i, got)
		}
	}
}

// A leaf that happens to be blank is still Steam's.
func TestWriteAutoLoginKeepsAnEmptyValue(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".local", "share", "Steam")
	path := registryVDFPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := "\"Registry\"\n{\n\t\"HKCU\"\n\t{\n\t\t\"Software\"\n\t\t{\n\t\t\t\"Valve\"\n\t\t\t{\n\t\t\t\t\"Steam\"\n\t\t\t\t{\n\t\t\t\t\t\"LastGameNameUsed\"\t\t\"\"\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n}\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeAutoLogin(root, "alpha"); err != nil {
		t.Fatalf("writeAutoLogin: %v", err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), `"LastGameNameUsed"`) {
		t.Errorf("an empty-valued key Steam owns was dropped:\n%s", got)
	}
}
