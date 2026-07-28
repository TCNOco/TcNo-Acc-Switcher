package steamguard

import (
	"crypto/rand"
	"encoding/base64"
	"strings"

	"TcNo-Acc-Switcher/internal/steamguard/capability"
	"TcNo-Acc-Switcher/internal/steamguard/vault"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const (
	confirmationsWindowName      = "steam-guard-confirmations"
	confirmationsCapabilityScope = "steamguard-confirmations"
	ConfirmationsGrantEvent      = "steamguard:confirmations-grant"
	ConfirmationsContextEvent    = "steamguard:confirmations-context-changed"
	LoginAgainRequestEvent       = "steamguard:login-again-request"
)

// registerLoginAgainHandoff brings the main window forward when the
// confirmations window asks the user to sign in again. The Steam Guard modal
// itself is opened by the main window's frontend listener for the same event.
// The confirmations window closes so the user signs in via the main app.
func registerLoginAgainHandoff() {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.On(LoginAgainRequestEvent, func(*application.CustomEvent) {
		if confirmationsWin, ok := app.Window.GetByName(confirmationsWindowName); ok && confirmationsWin != nil {
			confirmationsWin.Close()
		}
		window, ok := app.Window.GetByName(mainWindowName)
		if !ok || window == nil {
			return
		}
		window.Show()
		window.Restore()
		window.Focus()
	})
}

type ConfirmationsGrant struct {
	Capability string `json:"capability"`
	AccountID  string `json:"accountId"`
	RequestID  string `json:"requestId"`
}

// confirmationsWindowTitle renders the window caption for an account. The
// username is a display string only; an unknown account falls back to the
// generic caption rather than showing an empty prefix.
func confirmationsWindowTitle(username string) string {
	username = strings.TrimSpace(username)
	if username == "" {
		return "Steam Guard Confirmations"
	}
	return username + " - Confirmations"
}

func confirmationsWindowOptions(username string) application.WebviewWindowOptions {
	return application.WebviewWindowOptions{
		Name:   confirmationsWindowName,
		Title:  confirmationsWindowTitle(username),
		URL:    "/#/steam/confirmations",
		Width:  490,
		Height: 700,
		// The list scrolls, so the window only needs room for the header and the
		// action bar; the default is taller than the minimum to show several rows.
		MinWidth:                   490,
		MinHeight:                  455,
		Frameless:                  true,
		EnableFileDrop:             false,
		DevToolsEnabled:            false,
		DefaultContextMenuDisabled: true,
		ContentProtectionEnabled:   contentProtectionEnabled(),
		BackgroundColour:           application.NewRGB(27, 38, 54),
		Permissions: map[application.PermissionType]application.Permission{
			application.PermissionCamera:        application.PermissionDeny,
			application.PermissionMicrophone:    application.PermissionDeny,
			application.PermissionGeolocation:   application.PermissionDeny,
			application.PermissionNotifications: application.PermissionDeny,
			application.PermissionClipboardRead: application.PermissionDeny,
		},
	}
}

// OpenConfirmations validates the main-window modal grant before creating or
// focusing the single protected confirmations window.
func (s *Service) OpenConfirmations(accountID, modalToken string) error {
	accountID = strings.TrimSpace(accountID)
	s.mu.Lock()
	v, err := s.requireVaultLocked()
	if err == nil {
		err = s.authorizeModalLocked(v, accountID, modalToken)
	}
	if err == nil && v.IsLocked() {
		err = vault.ErrLocked
	}
	var generation, username string
	if err == nil {
		generation = v.Generation()
		// Best-effort: a missing name only costs the window its caption prefix.
		username = accountNameForSteamID64Locked(v, accountID)
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	app := application.Get()
	if app == nil || app.Window == nil {
		return ErrSensitiveView
	}

	instanceID, err := randomWindowInstanceID()
	if err != nil {
		return err
	}
	s.confirmationWindowMu.Lock()
	s.resetConfirmationLocked(false)
	s.confirmationAccountID = accountID
	s.confirmationGeneration = generation
	s.confirmationInstanceID = instanceID
	if existing, ok := app.Window.GetByName(confirmationsWindowName); ok {
		s.capabilities.RevokeWindow(confirmationsWindowName)
		// Reused window: retitle for the account it now serves.
		existing.SetTitle(confirmationsWindowTitle(username))
		existing.DispatchWailsEvent(&application.CustomEvent{Name: ConfirmationsContextEvent, Sender: "native"})
		existing.Show()
		existing.Restore()
		existing.Focus()
		s.confirmationWindowMu.Unlock()
		return nil
	}
	window := app.Window.NewWithOptions(confirmationsWindowOptions(username))
	window.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		s.confirmationWindowMu.Lock()
		s.resetConfirmationLocked(true)
		s.capabilities.RevokeWindow(confirmationsWindowName)
		s.confirmationWindowMu.Unlock()
		// None of the cached icons are needed once the window is gone.
		s.confirmationIcons.Clear()
	})
	s.confirmationWindowMu.Unlock()
	return nil
}

// RequestConfirmationsCapability sends a bearer grant only to the protected
// confirmations window after it has mounted its listener.
func (s *Service) RequestConfirmationsCapability(requestID string) error {
	requestID = strings.TrimSpace(requestID)
	if len(requestID) < 16 || len(requestID) > 128 {
		return capability.ErrInvalidCapability
	}
	s.confirmationWindowMu.Lock()
	defer s.confirmationWindowMu.Unlock()
	if s.confirmationAccountID == "" || s.confirmationInstanceID == "" || s.capabilities == nil {
		return capability.ErrInvalidCapability
	}
	s.mu.Lock()
	v, exists, err := s.openVaultLocked()
	if err == nil && (!exists || v.IsLocked() || v.Generation() != s.confirmationGeneration) {
		err = capability.ErrInvalidCapability
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	binding := capability.Binding{
		WindowName:      confirmationsWindowName,
		AccountID:       s.confirmationAccountID,
		Scope:           confirmationsCapabilityScope,
		LeaseID:         s.confirmationInstanceID,
		VaultGeneration: s.confirmationGeneration,
	}
	token, err := s.capabilities.Issue(binding)
	if err != nil {
		return err
	}
	app := application.Get()
	if app == nil || app.Window == nil {
		s.capabilities.Revoke(token)
		return ErrSensitiveView
	}
	window, ok := app.Window.GetByName(confirmationsWindowName)
	if !ok {
		s.capabilities.Revoke(token)
		return ErrSensitiveView
	}
	window.DispatchWailsEvent(&application.CustomEvent{
		Name:   ConfirmationsGrantEvent,
		Sender: "native",
		Data: ConfirmationsGrant{
			Capability: token,
			AccountID:  s.confirmationAccountID,
			RequestID:  requestID,
		},
	})
	return nil
}

func (s *Service) ReleaseConfirmationsCapability(token string) {
	if s.capabilities != nil {
		s.capabilities.Revoke(token)
	}
}

// accountNameForSteamID64Locked resolves the vault account name for a SteamID64.
// Callers must hold s.mu and pass an unlocked vault. It returns "" for any
// lookup or decode failure: the name is used for display only.
func accountNameForSteamID64Locked(v *vault.Vault, steamID64 string) string {
	if v == nil || steamID64 == "" {
		return ""
	}
	records, err := v.List()
	if err != nil {
		return ""
	}
	for _, record := range records {
		if record.SteamID64 != steamID64 {
			continue
		}
		account, err := accountFromRecord(v, record.ID)
		if err != nil {
			return ""
		}
		return account.AccountName
	}
	return ""
}

func randomWindowInstanceID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
