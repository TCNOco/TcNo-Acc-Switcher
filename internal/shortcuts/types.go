package shortcuts

// UpdatedEvent is emitted after scan or order changes.
const UpdatedEvent = "shortcuts-updated"

// FilesDroppedEvent is emitted from main when the OS drops files onto the window (absolute paths).
const FilesDroppedEvent = "files-dropped"

// ListPayload is the Wails event payload for [UpdatedEvent].
type ListPayload struct {
	PlatformKey string        `json:"platformKey"`
	Shortcuts   []ShortcutDTO `json:"shortcuts"`
}

// ShortcutDTO is one row for the footer game shortcut UI.
type ShortcutDTO struct {
	FileName      string `json:"fileName"`
	DisplayName   string `json:"displayName"`
	IconURL       string `json:"iconUrl"`
	Pinned        bool   `json:"pinned"`
	IsPlatformExe bool   `json:"isPlatformExe"`
	IsURL         bool   `json:"isUrl"`
}
