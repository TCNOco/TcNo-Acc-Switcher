//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// userFolderKinds is the order the picker lists the user's folders in.
var userFolderKinds = []string{
	"home", "desktop", "documents", "downloads", "pictures", "music", "videos",
}

var knownFolderIDs = map[string]*windows.KNOWNFOLDERID{
	"home":      windows.FOLDERID_Profile,
	"desktop":   windows.FOLDERID_Desktop,
	"documents": windows.FOLDERID_Documents,
	"downloads": windows.FOLDERID_Downloads,
	"pictures":  windows.FOLDERID_Pictures,
	"music":     windows.FOLDERID_Music,
	"videos":    windows.FOLDERID_Videos,
}

// userFolderPath asks Windows where a folder actually is. Reading it from the
// profile directory would miss the redirection OneDrive and roaming profiles
// apply.
func userFolderPath(kind string) (string, error) {
	id, ok := knownFolderIDs[kind]
	if !ok {
		return "", nil
	}
	return windows.KnownFolderPath(id, 0)
}

// driveRoots lists the drive letters that are actually mounted.
func driveRoots() []FsRoot {
	roots := make([]FsRoot, 0, 26)
	for _, letter := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		path := string(letter) + `:\`
		fi, err := os.Stat(path)
		if err != nil || !fi.IsDir() {
			continue
		}
		roots = append(roots, FsRoot{Path: path, Label: path, Kind: driveRootKind(path)})
	}
	return roots
}

func driveRootKind(root string) string {
	name, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return RootKindDrive
	}
	if windows.GetDriveType(name) == windows.DRIVE_REMOTE {
		return RootKindNetworkDrive
	}
	return RootKindDrive
}
