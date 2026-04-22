package shortcuts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"TcNo-Acc-Switcher/internal/basic"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/steam"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Service exposes game shortcut operations to the Wails frontend.
type Service struct {
	ps *platform.PlatformService

	mu       sync.Mutex
	scanBusy map[string]struct{}
}

// NewService constructs the shortcuts service.
func NewService(ps *platform.PlatformService) *Service {
	return &Service{
		ps:       ps,
		scanBusy: map[string]struct{}{},
	}
}

func (s *Service) buildDTOs(platformKey string) ([]ShortcutDTO, error) {
	platformKey = strings.TrimSpace(platformKey)
	entries, err := loadEntries(platformKey)
	if err != nil {
		return nil, err
	}
	out := make([]ShortcutDTO, 0, len(entries))
	for _, e := range entries {
		fn := strings.TrimSpace(e.FileName)
		if fn == "" {
			continue
		}
		low := strings.ToLower(fn)
		out = append(out, ShortcutDTO{
			FileName:      fn,
			DisplayName:   removeShortcutExt(fn),
			IconURL:       iconPublicURL(platformKey, fn),
			Pinned:        e.Pinned,
			IsPlatformExe: false,
			IsURL:         strings.HasSuffix(low, ".url"),
		})
	}
	return out, nil
}

func (s *Service) emitUpdated(platformKey string) {
	app := application.Get()
	if app == nil {
		return
	}
	list, err := s.buildDTOs(platformKey)
	if err != nil {
		list = nil
	}
	app.Event.Emit(UpdatedEvent, ListPayload{PlatformKey: platformKey, Shortcuts: list})
}

// ListShortcuts returns persisted shortcuts and icon URLs without scanning source folders.
func (s *Service) ListShortcuts(platformKey string) ([]ShortcutDTO, error) {
	return s.buildDTOs(strings.TrimSpace(platformKey))
}

// ScanShortcuts reconciles Start Menu / configured folders with the cache (async).
func (s *Service) ScanShortcuts(platformKey string) error {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return nil
	}
	s.mu.Lock()
	if _, ok := s.scanBusy[platformKey]; ok {
		s.mu.Unlock()
		return nil
	}
	s.scanBusy[platformKey] = struct{}{}
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.scanBusy, platformKey)
			s.mu.Unlock()
		}()
		_ = s.reconcile(platformKey)
	}()
	return nil
}

// RunShortcut launches a cached .lnk or .url. When selectedUniqueID is set and AlwaysSwapOnShortcut is enabled for the platform, swaps first (strict on failure).
func (s *Service) RunShortcut(platformKey, fileName string, admin bool, selectedUniqueID string) error {
	platformKey = strings.TrimSpace(platformKey)
	selectedUniqueID = strings.TrimSpace(selectedUniqueID)
	if selectedUniqueID != "" {
		ps, err := platform.LoadPlatformSettings(platformKey)
		if err != nil {
			return err
		}
		if ps.AlwaysSwapOnShortcut {
			if strings.EqualFold(platformKey, "Steam") {
				if err := steam.SwapToAccount(selectedUniqueID, -1); err != nil {
					return err
				}
			} else {
				if err := basic.SwapTo(basic.FlowDeps{PS: s.ps}, platformKey, selectedUniqueID); err != nil {
					return err
				}
			}
		}
	}
	return RunShortcut(platformKey, fileName, admin)
}

// CreateAccountShortcut writes a desktop shortcut with CLI args for this account.
func (s *Service) CreateAccountShortcut(platformKey, uniqueID, displayName, stateSuffix string) (string, error) {
	return CreateAccountShortcut(platformKey, uniqueID, displayName, stateSuffix)
}

// HideShortcut hides a shortcut (rename to _ignored).
func (s *Service) HideShortcut(platformKey, fileName string) error {
	if err := HideShortcut(platformKey, fileName); err != nil {
		return err
	}
	s.emitUpdated(strings.TrimSpace(platformKey))
	return nil
}

// SaveShortcutOrder persists pinned vs dropdown order (filenames only).
func (s *Service) SaveShortcutOrder(platformKey string, pinned, dropdown []string) error {
	platformKey = strings.TrimSpace(platformKey)
	cur, err := loadEntries(platformKey)
	if err != nil {
		return err
	}
	want := make(map[string]struct{}, len(cur))
	for _, e := range cur {
		want[strings.ToLower(strings.TrimSpace(e.FileName))] = struct{}{}
	}
	seen := make(map[string]struct{})
	var norm []platform.GameShortcutEntry
	for _, raw := range pinned {
		fn := filepath.Base(strings.TrimSpace(raw))
		if fn == "" {
			continue
		}
		low := strings.ToLower(fn)
		if _, ok := want[low]; !ok {
			return fmt.Errorf("unknown shortcut in pinned list: %s", fn)
		}
		if _, dup := seen[low]; dup {
			return fmt.Errorf("duplicate shortcut: %s", fn)
		}
		seen[low] = struct{}{}
		norm = append(norm, platform.GameShortcutEntry{FileName: fn, Pinned: true})
	}
	for _, raw := range dropdown {
		fn := filepath.Base(strings.TrimSpace(raw))
		if fn == "" {
			continue
		}
		low := strings.ToLower(fn)
		if _, ok := want[low]; !ok {
			return fmt.Errorf("unknown shortcut in dropdown list: %s", fn)
		}
		if _, dup := seen[low]; dup {
			return fmt.Errorf("duplicate shortcut: %s", fn)
		}
		seen[low] = struct{}{}
		norm = append(norm, platform.GameShortcutEntry{FileName: fn, Pinned: false})
	}
	if len(seen) != len(want) {
		return fmt.Errorf("order must include every shortcut exactly once")
	}
	if err := saveEntries(platformKey, norm); err != nil {
		return err
	}
	s.emitUpdated(platformKey)
	return nil
}

// OpenShortcutFolder opens LoginCache/<platform>/Shortcuts in the file manager.
func (s *Service) OpenShortcutFolder(platformKey string) error {
	platformKey = strings.TrimSpace(platformKey)
	root, err := paths.LoginCacheDir(platformKey)
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "Shortcuts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return platform.OpenPathInFileManager(dir)
}
