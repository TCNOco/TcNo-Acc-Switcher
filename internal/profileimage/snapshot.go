package profileimage

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// cacheExtensions is the probe order shared by the per-account lookups and the
// snapshot. The first entry that exists wins, so the order is part of the
// contract rather than an implementation detail.
var cacheExtensions = []string{"jpg", "jpeg", "png", "webp", "gif", "webm", "mp4"}

// Lookup answers avatar-cache questions for one platform. Snapshot implements
// it from a single directory read; DirectLookup implements it by hitting the
// filesystem per call.
//
// Which one a caller wants is a correctness decision, not a performance one.
// Code that only reads, such as building a list for the UI, wants a Snapshot.
// Code that writes avatars while it runs, such as the background refresh, must
// use DirectLookup, because a snapshot taken before a download would still
// report the pre-download state.
type Lookup interface {
	FindCached(accountID string) (string, bool)
	CachedFilePath(accountID string) (string, bool)
	HasManualProfileMarker(accountID string) bool
	OlderThanDays(accountID string, days int) bool
}

// DirectLookup satisfies Lookup with the live per-account functions, so it
// always observes the current filesystem.
type DirectLookup string

func (d DirectLookup) FindCached(accountID string) (string, bool) {
	return FindCached(string(d), accountID)
}

func (d DirectLookup) CachedFilePath(accountID string) (string, bool) {
	return CachedFilePath(string(d), accountID)
}

func (d DirectLookup) HasManualProfileMarker(accountID string) bool {
	return HasManualProfileMarker(string(d), accountID)
}

// OlderThanDays reports a missing cache entry as stale, matching how the
// account list treats an account with nothing cached.
func (d DirectLookup) OlderThanDays(accountID string, days int) bool {
	path, ok := CachedFilePath(string(d), accountID)
	if !ok {
		return true
	}
	return FileOlderThanDays(path, days)
}

var (
	_ Lookup = (*Snapshot)(nil)
	_ Lookup = DirectLookup("")
)

// Snapshot answers the same questions as FindCached, CachedFilePath and
// HasManualProfileMarker, for every account in a platform at once.
//
// The per-account calls each walk the extension list with a stat per candidate,
// so resolving a list costs roughly fifteen syscalls per account. One directory
// read answers all of them: on Windows the directory scan already carries the
// metadata, so entry timestamps come back without further syscalls.
//
// A snapshot is a point-in-time view. Take one per list build; do not cache it
// across user actions that write avatars.
type Snapshot struct {
	platformKey string
	dir         string
	images      map[string]cachedImage
	markers     map[string]struct{}
}

type cachedImage struct {
	ext     string
	modTime time.Time
}

// NewSnapshot reads a platform's profile directory once. A missing directory is
// not an error; the snapshot simply reports nothing cached, matching what the
// per-account lookups do when they find no files.
func NewSnapshot(platformKey string) (*Snapshot, error) {
	dir, err := ProfileDir(platformKey)
	if err != nil {
		return nil, err
	}
	snapshot := &Snapshot{
		platformKey: platformKey,
		dir:         dir,
		images:      map[string]cachedImage{},
		markers:     map[string]struct{}{},
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return snapshot, nil
		}
		return nil, err
	}

	rank := make(map[string]int, len(cacheExtensions))
	for i, ext := range cacheExtensions {
		rank[ext] = i
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if stem, ok := strings.CutSuffix(name, ManualProfileMarkerSuffix); ok {
			snapshot.markers[stem] = struct{}{}
			continue
		}
		suffix := filepath.Ext(name)
		position, known := rank[strings.TrimPrefix(suffix, ".")]
		if !known {
			continue
		}
		stem := strings.TrimSuffix(name, suffix)
		// Keep whichever extension the per-account probe would have hit first.
		if existing, seen := snapshot.images[stem]; seen && rank[existing.ext] <= position {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		snapshot.images[stem] = cachedImage{ext: strings.TrimPrefix(suffix, "."), modTime: info.ModTime()}
	}
	return snapshot, nil
}

// FindCached reports the public URL of a cached image, matching the package
// function of the same name.
func (s *Snapshot) FindCached(accountID string) (string, bool) {
	if s == nil {
		return "", false
	}
	image, ok := s.images[accountID]
	if !ok {
		return "", false
	}
	return PublicPath(s.platformKey, accountID, image.ext), true
}

// CachedFilePath reports the on-disk path of a cached image.
func (s *Snapshot) CachedFilePath(accountID string) (string, bool) {
	if s == nil {
		return "", false
	}
	image, ok := s.images[accountID]
	if !ok {
		return "", false
	}
	return filepath.Join(s.dir, accountID+"."+image.ext), true
}

// HasManualProfileMarker reports whether the account has a manual avatar lock.
func (s *Snapshot) HasManualProfileMarker(accountID string) bool {
	if s == nil {
		return false
	}
	_, ok := s.markers[strings.TrimSpace(accountID)]
	return ok
}

// OlderThanDays mirrors FileOlderThanDays for a cached image, including its
// treatment of a non-positive day count and of an account with nothing cached
// as "stale".
func (s *Snapshot) OlderThanDays(accountID string, days int) bool {
	if days <= 0 || s == nil {
		return true
	}
	image, ok := s.images[accountID]
	if !ok {
		return true
	}
	return time.Since(image.modTime) > time.Duration(days)*24*time.Hour
}
