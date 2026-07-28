package main

import (
	"os"
	"testing"
)

func TestListRootsPutsUserFoldersAfterDrives(t *testing.T) {
	f := &FilesystemService{}
	roots, err := f.ListRoots()
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) == 0 {
		t.Fatal("no roots listed")
	}

	lastDrive := -1
	firstUserFolder := -1
	seen := make(map[string]struct{}, len(roots))
	for i, root := range roots {
		if root.Path == "" || root.Label == "" || root.Kind == "" {
			t.Fatalf("root %d is incomplete: %+v", i, root)
		}
		if _, dup := seen[root.Path]; dup {
			t.Fatalf("root %q listed twice", root.Path)
		}
		seen[root.Path] = struct{}{}
		// A root the picker cannot open is worse than one it never offered.
		fi, statErr := os.Stat(root.Path)
		if statErr != nil || !fi.IsDir() {
			t.Fatalf("root %q is not a readable directory: %v", root.Path, statErr)
		}
		if root.Kind == RootKindDrive || root.Kind == RootKindNetworkDrive {
			if firstUserFolder >= 0 {
				t.Fatalf("drive %q listed after user folder %q", root.Path, roots[firstUserFolder].Path)
			}
			lastDrive = i
			continue
		}
		if firstUserFolder < 0 {
			firstUserFolder = i
		}
	}
	if lastDrive < 0 {
		t.Fatal("no drive listed")
	}
}

func TestUserFolderRootsAreLabelledByName(t *testing.T) {
	roots := userFolderRoots()
	if len(roots) == 0 {
		t.Skip("no user folders resolved on this machine")
	}
	for _, root := range roots {
		if root.Kind == RootKindDrive || root.Kind == RootKindNetworkDrive {
			t.Fatalf("user folder %q reported as a drive", root.Path)
		}
		// The label names the folder; showing the whole path would defeat
		// offering it as a shortcut.
		if len(root.Label) >= len(root.Path) {
			t.Fatalf("user folder label %q is not a name within %q", root.Label, root.Path)
		}
	}
}
