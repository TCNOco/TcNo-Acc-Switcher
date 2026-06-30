package shortcuts

// UpdatedEvent is emitted after scan or order changes.
const UpdatedEvent = "shortcuts-updated"

// FilesDroppedEvent is emitted from main when the OS drops files onto the window.
const FilesDroppedEvent = "files-dropped"

// ListPayload is the Wails event payload for [UpdatedEvent].
type ListPayload struct {
	PlatformKey string        `json:"platformKey"`
	Shortcuts   []ShortcutDTO `json:"shortcuts"`
}

type FileDropTargetDetails struct {
	ElementID string   `json:"elementId"`
	ClassList []string `json:"classList"`
	X         int      `json:"x"`
	Y         int      `json:"y"`
}

type FilesDroppedPayload struct {
	Files  []string              `json:"files"`
	Target FileDropTargetDetails `json:"target"`
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
