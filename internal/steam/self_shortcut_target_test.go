package steam

import "testing"

func TestSelfShortcutTargetForEachInstallMethod(t *testing.T) {
	const nativeRoot = "/home/u/.local/share/Steam"
	const flatpakRoot = "/home/u/.var/app/com.valvesoftware.Steam/.local/share/Steam"

	cases := []struct {
		name      string
		goos      string
		exePath   string
		steamRoot string
		env       map[string]string
		present   []string

		wantExe      string
		wantStartDir string
		wantLaunch   string
		wantIcon     string
		wantBinary   string
		wantFlatpak  bool
	}{
		{
			name:         "windows installed",
			goos:         "windows",
			exePath:      `C:\Program Files\TcNo Account Switcher\TcNo-Acc-Switcher.exe`,
			steamRoot:    `C:\Program Files (x86)\Steam`,
			wantExe:      `"C:\Program Files\TcNo Account Switcher\TcNo-Acc-Switcher.exe"`,
			wantStartDir: `"C:\Program Files\TcNo Account Switcher\"`,
			// No icon ships beside the exe; Steam reads it out of the binary.
			wantIcon:   `C:\Program Files\TcNo Account Switcher\TcNo-Acc-Switcher.exe`,
			wantBinary: `C:\Program Files\TcNo Account Switcher\TcNo-Acc-Switcher.exe`,
		},
		{
			name:         "linux package install",
			goos:         "linux",
			exePath:      "/usr/local/bin/tcno-acc-switcher",
			steamRoot:    nativeRoot,
			present:      []string{"/usr/local/share/icons/hicolor/256x256/apps/tcno-acc-switcher.png"},
			wantExe:      `"/usr/local/bin/tcno-acc-switcher"`,
			wantStartDir: `"/usr/local/bin/"`,
			wantIcon:     "/usr/local/share/icons/hicolor/256x256/apps/tcno-acc-switcher.png",
			wantBinary:   "/usr/local/bin/tcno-acc-switcher",
		},
		{
			name:         "linux loose binary with no icon installed",
			goos:         "linux",
			exePath:      "/home/u/apps/tcno-acc-switcher",
			steamRoot:    nativeRoot,
			wantExe:      `"/home/u/apps/tcno-acc-switcher"`,
			wantStartDir: `"/home/u/apps/"`,
			wantIcon:     "",
			wantBinary:   "/home/u/apps/tcno-acc-switcher",
		},
		{
			name:      "appimage points at the .AppImage, not its mount",
			goos:      "linux",
			exePath:   "/tmp/.mount_tcnoAbCdEf/usr/bin/tcno-acc-switcher",
			steamRoot: nativeRoot,
			env: map[string]string{
				"APPIMAGE": "/home/u/Applications/tcno-acc-switcher-x86_64.AppImage",
				"APPDIR":   "/tmp/.mount_tcnoAbCdEf",
			},
			present:      []string{"/home/u/Applications/tcno-acc-switcher-x86_64.AppImage"},
			wantExe:      `"/home/u/Applications/tcno-acc-switcher-x86_64.AppImage"`,
			wantStartDir: `"/home/u/Applications/"`,
			wantBinary:   "/home/u/Applications/tcno-acc-switcher-x86_64.AppImage",
		},
		{
			name:      "appimage whose $APPIMAGE has gone falls back to the mount",
			goos:      "linux",
			exePath:   "/tmp/.mount_tcnoAbCdEf/usr/bin/tcno-acc-switcher",
			steamRoot: nativeRoot,
			env: map[string]string{
				"APPIMAGE": "/home/u/Applications/moved-away.AppImage",
			},
			wantExe:      `"/tmp/.mount_tcnoAbCdEf/usr/bin/tcno-acc-switcher"`,
			wantStartDir: `"/tmp/.mount_tcnoAbCdEf/usr/bin/"`,
			wantBinary:   "/tmp/.mount_tcnoAbCdEf/usr/bin/tcno-acc-switcher",
		},
		{
			name:         "macos app bundle launches the inner binary",
			goos:         "darwin",
			exePath:      "/Applications/TcNo-Acc-Switcher.app/Contents/MacOS/TcNo-Acc-Switcher",
			steamRoot:    "/Users/u/Library/Application Support/Steam",
			present:      []string{"/Applications/TcNo-Acc-Switcher.app/Contents/Resources/icons.icns"},
			wantExe:      `"/Applications/TcNo-Acc-Switcher.app/Contents/MacOS/TcNo-Acc-Switcher"`,
			wantStartDir: `"/Applications/TcNo-Acc-Switcher.app/Contents/MacOS/"`,
			wantIcon:     "/Applications/TcNo-Acc-Switcher.app/Contents/Resources/icons.icns",
			wantBinary:   "/Applications/TcNo-Acc-Switcher.app/Contents/MacOS/TcNo-Acc-Switcher",
		},
		{
			name:         "flatpak steam has to be told to leave its sandbox",
			goos:         "linux",
			exePath:      "/usr/local/bin/tcno-acc-switcher",
			steamRoot:    flatpakRoot,
			present:      []string{"/usr/local/share/icons/hicolor/256x256/apps/tcno-acc-switcher.png"},
			wantExe:      `"/usr/bin/flatpak-spawn"`,
			wantStartDir: `"/usr/bin/"`,
			wantLaunch:   `--host "/usr/local/bin/tcno-acc-switcher"`,
			wantIcon:     "/usr/local/share/icons/hicolor/256x256/apps/tcno-acc-switcher.png",
			wantBinary:   "/usr/local/bin/tcno-acc-switcher",
			wantFlatpak:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := func(k string) string { return tc.env[k] }
			exists := func(p string) bool {
				for _, have := range tc.present {
					if have == p {
						return true
					}
				}
				return false
			}
			got := selfShortcutTargetFor(tc.goos, tc.exePath, tc.steamRoot, env, exists)
			if got.Exe != tc.wantExe {
				t.Errorf("Exe = %s, want %s", got.Exe, tc.wantExe)
			}
			if got.StartDir != tc.wantStartDir {
				t.Errorf("StartDir = %s, want %s", got.StartDir, tc.wantStartDir)
			}
			if got.LaunchOptions != tc.wantLaunch {
				t.Errorf("LaunchOptions = %q, want %q", got.LaunchOptions, tc.wantLaunch)
			}
			if got.Icon != tc.wantIcon {
				t.Errorf("Icon = %q, want %q", got.Icon, tc.wantIcon)
			}
			if got.Binary != tc.wantBinary {
				t.Errorf("Binary = %q, want %q", got.Binary, tc.wantBinary)
			}
			if got.Flatpak != tc.wantFlatpak {
				t.Errorf("Flatpak = %v, want %v", got.Flatpak, tc.wantFlatpak)
			}
		})
	}
}

func TestIsSelfBinaryPathAcceptsEveryInstallMethodsName(t *testing.T) {
	ours := []string{
		`C:\Program Files\TcNo Account Switcher\TcNo-Acc-Switcher.exe`,
		`"C:\Program Files\TcNo Account Switcher\TcNo-Acc-Switcher.exe"`,
		"/usr/local/bin/tcno-acc-switcher",
		"/home/u/Applications/tcno-acc-switcher-x86_64.AppImage",
		"/Applications/TcNo-Acc-Switcher.app/Contents/MacOS/TcNo-Acc-Switcher",
	}
	for _, p := range ours {
		if !isSelfBinaryPath(p) {
			t.Errorf("isSelfBinaryPath(%q) = false, want true", p)
		}
	}

	theirs := []string{
		"/usr/bin/steam",
		`"C:\Games\other.exe"`,
		"/usr/bin/flatpak-spawn",
		"",
	}
	for _, p := range theirs {
		if isSelfBinaryPath(p) {
			t.Errorf("isSelfBinaryPath(%q) = true, want false", p)
		}
	}
}
