//go:build darwin

package main

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// userFolderKinds is the order the picker lists the user's folders in.
var userFolderKinds = []string{
	"home", "desktop", "documents", "downloads", "pictures", "music", "videos",
}

// macOS names the video folder Movies; the rest match the other platforms.
var userFolderNames = map[string]string{
	"desktop":   "Desktop",
	"documents": "Documents",
	"downloads": "Downloads",
	"pictures":  "Pictures",
	"music":     "Music",
	"videos":    "Movies",
}

func userFolderPath(kind string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if kind == "home" {
		return home, nil
	}
	name, ok := userFolderNames[kind]
	if !ok {
		return "", nil
	}
	return filepath.Join(home, name), nil
}

// driveRoots is the startup disk plus whatever is mounted under /Volumes,
// which is where macOS puts external disks and mounted shares.
func driveRoots() []FsRoot {
	roots := []FsRoot{{Path: "/", Label: "/", Kind: RootKindDrive}}
	entries, err := os.ReadDir("/Volumes")
	if err != nil {
		return roots
	}
	for _, entry := range entries {
		path := filepath.Join("/Volumes", entry.Name())
		fi, statErr := os.Stat(path)
		if statErr != nil || !fi.IsDir() {
			continue
		}
		// The startup disk is symlinked into /Volumes under its own name.
		if resolved, linkErr := filepath.EvalSymlinks(path); linkErr == nil && resolved == "/" {
			continue
		}
		roots = append(roots, FsRoot{Path: path, Label: entry.Name(), Kind: driveRootKind(path)})
	}
	return roots
}

// driveRootKind asks the filesystem itself rather than matching on names: a
// share can be mounted anywhere and be called anything.
func driveRootKind(root string) string {
	var st unix.Statfs_t
	if err := unix.Statfs(root, &st); err != nil {
		return RootKindDrive
	}
	if st.Flags&unix.MNT_LOCAL == 0 {
		return RootKindNetworkDrive
	}
	return RootKindDrive
}
