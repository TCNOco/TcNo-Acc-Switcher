//go:build !windows && !darwin

package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// userFolderKinds is the order the picker lists the user's folders in.
var userFolderKinds = []string{
	"home", "desktop", "documents", "downloads", "pictures", "music", "videos",
}

// xdgKeys maps each kind to its XDG user directory and the fallback folder
// name to use when the directory is not configured.
var xdgKeys = map[string]struct{ key, fallback string }{
	"desktop":   {"XDG_DESKTOP_DIR", "Desktop"},
	"documents": {"XDG_DOCUMENTS_DIR", "Documents"},
	"downloads": {"XDG_DOWNLOAD_DIR", "Downloads"},
	"pictures":  {"XDG_PICTURES_DIR", "Pictures"},
	"music":     {"XDG_MUSIC_DIR", "Music"},
	"videos":    {"XDG_VIDEOS_DIR", "Videos"},
}

// userFolderPath resolves an XDG user directory. The names are localised and
// relocatable, so a guessed English path under the home directory is only the
// last resort.
func userFolderPath(kind string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if kind == "home" {
		return home, nil
	}
	entry, ok := xdgKeys[kind]
	if !ok {
		return "", nil
	}
	if fromEnv := strings.TrimSpace(os.Getenv(entry.key)); fromEnv != "" {
		return expandUserDir(fromEnv, home), nil
	}
	if configured := xdgUserDir(entry.key, home); configured != "" {
		return configured, nil
	}
	return filepath.Join(home, entry.fallback), nil
}

// xdgUserDir reads user-dirs.dirs, whose lines look like
// XDG_MUSIC_DIR="$HOME/Musique".
func xdgUserDir(key, home string) string {
	configHome := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	file, err := os.Open(filepath.Join(configHome, "user-dirs.dirs"))
	if err != nil {
		return ""
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		name, value, found := strings.Cut(line, "=")
		if !found || strings.TrimSpace(name) != key {
			continue
		}
		value = strings.TrimSpace(value)
		value = strings.TrimSuffix(strings.TrimPrefix(value, `"`), `"`)
		if value == "" {
			continue
		}
		return expandUserDir(value, home)
	}
	return ""
}

func expandUserDir(value, home string) string {
	value = strings.TrimSpace(value)
	switch {
	case value == "$HOME" || value == "~":
		return home
	case strings.HasPrefix(value, "$HOME/"):
		return filepath.Join(home, value[len("$HOME/"):])
	case strings.HasPrefix(value, "~/"):
		return filepath.Join(home, value[2:])
	}
	return value
}

// driveRoots is the filesystem root plus mounted media. Removable drives land
// under /media or /run/media depending on the distribution, and /mnt is where
// manual mounts go.
func driveRoots() []FsRoot {
	mounts := mountPoints()
	roots := []FsRoot{{Path: "/", Label: "/", Kind: kindForMount(mounts, "/")}}

	var parents []string
	if user := strings.TrimSpace(os.Getenv("USER")); user != "" {
		parents = append(parents, filepath.Join("/media", user), filepath.Join("/run/media", user))
	}
	parents = append(parents, "/media", "/run/media", "/mnt")

	seen := make(map[string]struct{})
	for _, parent := range parents {
		entries, err := os.ReadDir(parent)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			path := filepath.Join(parent, entry.Name())
			if _, dup := seen[path]; dup {
				continue
			}
			// Only actual mounts: /media/<user> holds the mounts, and an empty
			// mount folder left behind is not a drive.
			isRemote, mounted := mounts[path]
			if !mounted {
				continue
			}
			if fi, statErr := os.Stat(path); statErr != nil || !fi.IsDir() {
				continue
			}
			seen[path] = struct{}{}
			kind := RootKindDrive
			if isRemote {
				kind = RootKindNetworkDrive
			}
			roots = append(roots, FsRoot{Path: path, Label: entry.Name(), Kind: kind})
		}
	}
	return roots
}

func kindForMount(mounts map[string]bool, path string) string {
	if mounts[path] {
		return RootKindNetworkDrive
	}
	return RootKindDrive
}

// mountPoints maps every mount point to whether its filesystem is a network
// one, read from the kernel rather than guessed from the path.
func mountPoints() map[string]bool {
	file, err := os.Open("/proc/self/mounts")
	if err != nil {
		return map[string]bool{}
	}
	defer func() { _ = file.Close() }()

	remote := map[string]bool{
		"nfs": true, "nfs4": true, "cifs": true, "smb3": true, "smbfs": true,
		"afs": true, "sshfs": true, "ftpfs": true, "davfs": true, "fuse.sshfs": true,
		"fuse.davfs": true, "fuse.smbnetfs": true, "9p": true,
	}
	out := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		// Mount points are escaped in octal for spaces and tabs.
		point := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\134`, `\`).Replace(fields[1])
		out[point] = remote[fields[2]]
	}
	return out
}

func driveRootKind(root string) string {
	return kindForMount(mountPoints(), root)
}
