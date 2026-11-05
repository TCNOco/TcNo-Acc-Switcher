package basic

import (
	"fmt"
	"strings"

	"TcNo-Acc-Switcher/internal/paths"
)

func validateAccountSaveName(ids map[string]string, uniqueID, accountName string) error {
	for existingID, existingName := range ids {
		if strings.EqualFold(strings.TrimSpace(existingID), strings.TrimSpace(uniqueID)) {
			continue
		}
		if strings.EqualFold(paths.WindowsFileName(strings.TrimSpace(existingName), 200), accountName) {
			return fmt.Errorf("account name %q already belongs to another account; choose a different name", accountName)
		}
	}
	return nil
}

func shouldRestoreSavedSession(fc FlowContext, currentID, requestedID string, readErr error) bool {
	return !isCredentialSessionFlow(fc) || readErr != nil || strings.TrimSpace(currentID) == "" ||
		!strings.EqualFold(strings.TrimSpace(currentID), strings.TrimSpace(requestedID))
}
