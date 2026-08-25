package platform

import (
	"encoding/json"
	"errors"
	"strings"
)

const SameAsLoginFiles = "SAME_AS_LOGIN_FILES"

type DescriptorExtras struct {
	CachePaths              []string          `json:"CachePaths,omitempty"`
	BackupFolders           map[string]string `json:"BackupFolders,omitempty"`
	Variables               map[string]string `json:"variables,omitempty"`
	ProfilePicPath          string            `json:"ProfilePicPath,omitempty"`
	ProfilePicFromFile      string            `json:"ProfilePicFromFile,omitempty"`
	ProfilePicRegex         string            `json:"ProfilePicRegex,omitempty"`
	BuiltInUsernameFile     string            `json:"BuiltInUsernameFile,omitempty"`
	BuiltInProfileImageFile string            `json:"BuiltInProfileImageFile,omitempty"`
	BuiltInUserId           string            `json:"BuiltInUserId,omitempty"`
	ShortcutFolders         []string          `json:"ShortcutFolders,omitempty"`
	ShortcutIgnore          []string          `json:"ShortcutIgnore,omitempty"`
	ShortcutIncludeMainExe  *bool             `json:"ShortcutIncludeMainExe,omitempty"`
	SearchStartMenuForIcon  bool              `json:"SearchStartMenuForIcon,omitempty"`
	BackupFileTypesInclude  []string          `json:"BackupFileTypesInclude,omitempty"`
	BackupFileTypesIgnore   []string          `json:"BackupFileTypesIgnore,omitempty"`
	ClosingMethod           string            `json:"ClosingMethod,omitempty"`
	ForceClosingMethod      bool              `json:"ForceClosingMethod,omitempty"`
	// QuitArgs are passed to the platform's own executable to ask it to shut itself down,
	// e.g. "-shutdown" for Steam. Use it when the platform ignores WM_CLOSE - a launcher that
	// minimises to tray on close never exits, so the graceful window expires on every switch
	// and the client is force-killed instead. Same tokenising as ExeExtraArgs.
	//
	// Only ever run against the executable this descriptor already launches, never an
	// arbitrary path: it must not be able to widen what a catalog can spawn.
	QuitArgs string `json:"QuitArgs,omitempty"`
}

type Descriptor struct {
	Identifiers              []string               `json:"Identifiers,omitempty"`
	ExeLocationDefault       ExeLocationDefaultList `json:"ExeLocationDefault,omitempty"`
	ExeExtraArgs             string                 `json:"ExeExtraArgs,omitempty"`
	GetPathFromShortcutNamed string                 `json:"GetPathFromShortcutNamed,omitempty"`
	ExesToEnd                []string               `json:"ExesToEnd,omitempty"`
	PathListToClear          []string               `json:"PathListToClear,omitempty"`
	LoginFiles               map[string]string      `json:"LoginFiles,omitempty"`
	AllFilesRequired         bool                   `json:"AllFilesRequired"`
	DisabledByDefault        bool                   `json:"DisabledByDefault"`
	ExitBeforeInteract       bool                   `json:"ExitBeforeInteract"`
	ExitBeforeSave           bool                   `json:"ExitBeforeSave"`
	RegDeleteOnClear         bool                   `json:"RegDeleteOnClear"`
	UniqueIdFile             string                 `json:"UniqueIdFile,omitempty"`
	UniqueIdMethod           string                 `json:"UniqueIdMethod,omitempty"`
	UniqueIdRegex            string                 `json:"UniqueIdRegex,omitempty"`
	Extras                   DescriptorExtras       `json:"Extras,omitempty"`
}

// ParseDescriptor expands PathListToClear entries equal to SAME_AS_LOGIN_FILES into LoginFiles keys.
func ParseDescriptor(raw []byte, platformKey string) (Descriptor, error) {
	entries, _, err := indexCatalog(raw)
	if err != nil {
		return Descriptor{}, err
	}
	if entries == nil {
		return Descriptor{}, errors.New("missing Platforms")
	}
	blob, ok := entries[platformKey]
	if !ok {
		return Descriptor{}, errors.New("unknown platform: " + platformKey)
	}
	// Unmarshalled fresh every call, so the maps and slices on the returned
	// descriptor belong to this caller alone.
	var d Descriptor
	if err := json.Unmarshal(blob, &d); err != nil {
		return Descriptor{}, err
	}
	d.expandPathListToClear()
	return d, nil
}

func (d *Descriptor) expandPathListToClear() {
	if d.LoginFiles == nil {
		d.LoginFiles = map[string]string{}
	}
	var out []string
	for _, p := range d.PathListToClear {
		p = strings.TrimSpace(p)
		if p == SameAsLoginFiles {
			for k := range d.LoginFiles {
				out = append(out, k)
			}
			continue
		}
		if p != "" {
			out = append(out, p)
		}
	}
	d.PathListToClear = out
}

func (d Descriptor) ToPlatformEntry() PlatformEntry {
	return PlatformEntry{
		ExeLocationDefault:       append(ExeLocationDefaultList(nil), d.ExeLocationDefault...),
		GetPathFromShortcutNamed: d.GetPathFromShortcutNamed,
		ExesToEnd:                d.ExesToEnd,
	}
}
