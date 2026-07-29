package legacyinstall

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Entry is one leftover file or folder found next to the executable.
type Entry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Bytes int64  `json:"bytes"`
	// Preserve renames this entry to <name>.old instead of deleting it.
	Preserve bool `json:"preserve"`
}

// Report is the result of scanning an install directory for C# leftovers.
type Report struct {
	ExeDir  string   `json:"exeDir"`
	Markers []string `json:"markers"`
	Entries []Entry  `json:"entries"`
	Bytes   int64    `json:"bytes"`
}

// Found reports whether the directory holds a recognisable C# install worth cleaning.
func (r Report) Found() bool { return len(r.Markers) > 0 && len(r.Entries) > 0 }

// Count returns the number of top-level entries queued for removal.
func (r Report) Count() int { return len(r.Entries) }

// Detect scans dir - the folder holding the running executable - for files left
// behind by the C# release. It reports nothing unless a C#-only marker is
// present, and only ever lists names from the release manifest.
func Detect(dir string) Report {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return Report{}
	}
	dir = filepath.Clean(dir)
	rep := Report{ExeDir: dir}

	for _, name := range legacyMarkers {
		if st, err := os.Stat(filepath.Join(dir, name)); err == nil && !st.IsDir() {
			rep.Markers = append(rep.Markers, name)
		}
	}
	if len(rep.Markers) == 0 {
		return rep
	}

	for _, name := range legacyFiles {
		if isKept(name) {
			continue
		}
		path := filepath.Join(dir, name)
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			continue
		}
		rep.Entries = append(rep.Entries, Entry{
			Name:     name,
			Path:     path,
			Bytes:    st.Size(),
			Preserve: isPreserved(name),
		})
		rep.Bytes += st.Size()
	}

	for _, d := range legacyDirs {
		if isKept(d.name) {
			continue
		}
		path := filepath.Join(dir, d.name)
		st, err := os.Stat(path)
		if err != nil || !st.IsDir() {
			continue
		}
		if !matchesSignature(path, d) {
			continue
		}
		size := dirSize(path)
		rep.Entries = append(rep.Entries, Entry{Name: d.name, Path: path, IsDir: true, Bytes: size})
		rep.Bytes += size
	}

	return rep
}

func isKept(name string) bool {
	for _, keep := range keepNames {
		if strings.EqualFold(keep, name) {
			return true
		}
	}
	return false
}

// matchesSignature reports whether path looks like the C# build's copy of the
// folder rather than something else that happens to share the name.
func matchesSignature(path string, d legacyDir) bool {
	for _, sig := range d.signature {
		rel := filepath.FromSlash(sig)
		if _, err := os.Stat(filepath.Join(path, rel)); err == nil {
			return true
		}
	}
	if d.anySubdirFile == "" {
		return false
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(path, e.Name(), d.anySubdirFile)); err == nil {
			return true
		}
	}
	return false
}

func dirSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil //nolint:nilerr // a partial size is fine; this only feeds a display string
		}
		if info, ierr := entry.Info(); ierr == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// Writable reports whether the current process can delete from dir without
// elevation. Probing beats guessing from the path: a portable install under the
// user's own folders needs no UAC prompt, one under Program Files does.
func Writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".tcno-write-probe-")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}
