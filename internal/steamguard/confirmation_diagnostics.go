package steamguard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/steamguard/confirmationapi"
)

// confirmationDumpEnv opts into writing confirmation payloads to disk. It is off
// unless explicitly set, because these payloads describe live trades.
const confirmationDumpEnv = "TCNO_STEAMGUARD_CONFIRMATION_DUMP"

// confirmationDumpEnabled reports whether the operator asked for dumps this run.
func confirmationDumpEnabled() bool {
	return envSet(confirmationDumpEnv)
}

// confirmationDump is what one selection writes out: everything Steam sent about
// a confirmation, so the shape of a real trade or listing can be studied instead
// of guessed at.
//
// The confirmation's id and nonce are deliberately absent. Together they are the
// decision keys — anyone holding them alongside a session can accept the trade —
// and nothing about the payload's shape needs them. Handle is the same SHA-256
// digest the rest of the code uses to refer to a confirmation.
type confirmationDump struct {
	CapturedAt   string                  `json:"capturedAt"`
	Handle       string                  `json:"handle"`
	Type         int                     `json:"type"`
	TypeName     string                  `json:"typeName,omitempty"`
	TypeLabel    string                  `json:"typeLabel"`
	Title        string                  `json:"title"`
	Summary      []string                `json:"summary"`
	Icon         string                  `json:"icon,omitempty"`
	CreationTime int64                   `json:"creationTime,omitempty"`
	AcceptLabel  string                  `json:"acceptLabel"`
	DenyLabel    string                  `json:"denyLabel"`
	ParsedFields []confirmationDumpField `json:"parsedFields"`
	DetailsHTML  string                  `json:"detailsHtml"`
}

type confirmationDumpField struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// dumpConfirmation writes one payload under the Steam Guard settings folder and
// returns the path written, or "" when dumps are off or the write fails. A failed
// dump never affects the operation that triggered it.
func dumpConfirmation(item confirmationapi.Item, details confirmationapi.Details) string {
	if !confirmationDumpEnabled() {
		return ""
	}
	directory, err := confirmationDumpDir()
	if err != nil {
		confirmationsLogger().Warn("confirmation dump directory unavailable", "error", err)
		return ""
	}
	dump := confirmationDump{
		CapturedAt:   time.Now().UTC().Format(time.RFC3339),
		Handle:       item.Handle,
		Type:         item.Type,
		TypeName:     item.TypeName,
		TypeLabel:    item.TypeLabel,
		Title:        item.Title,
		Summary:      item.Summary,
		Icon:         item.Icon,
		CreationTime: item.CreationTime,
		AcceptLabel:  item.AcceptLabel,
		DenyLabel:    item.DenyLabel,
		DetailsHTML:  string(details.Raw),
	}
	for _, field := range details.Fields {
		dump.ParsedFields = append(dump.ParsedFields, confirmationDumpField{Label: field.Label, Value: field.Value})
	}
	encoded, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		confirmationsLogger().Warn("confirmation dump could not be encoded", "error", err)
		return ""
	}
	name := time.Now().UTC().Format("20060102-150405.000") + ".json"
	path := filepath.Join(directory, name)
	// 0600: the payload names trade counterparties and item inventories.
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		confirmationsLogger().Warn("confirmation dump could not be written", "error", err)
		return ""
	}
	confirmationsLogger().Info("confirmation payload written for analysis", "path", path)
	return path
}

func confirmationDumpDir() (string, error) {
	settings, err := paths.SettingsDir()
	if err != nil {
		return "", err
	}
	directory := filepath.Join(settings, "SteamGuard", "diagnostics")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", err
	}
	return directory, nil
}
