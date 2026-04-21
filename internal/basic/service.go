package basic

import (
	"os"
	"strings"
	"sync"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/profileimage"
)

// BasicService exposes generic platform account switching to the Wails frontend.
type BasicService struct {
	mu sync.Mutex
	PS *platform.PlatformService
}

func NewBasicService(ps *platform.PlatformService) *BasicService {
	return &BasicService{PS: ps}
}

func (b *BasicService) deps() FlowDeps {
	return FlowDeps{PS: b.PS}
}

// AccountDTO is one row in the generic account list.
type AccountDTO struct {
	PlatformKey string `json:"platformKey"`
	UniqueID    string `json:"uniqueId"`
	DisplayName string `json:"displayName"`
	ImageURL    string `json:"imageUrl"`
	Note        string `json:"note"`
}

// GetAccounts returns merged ids.json + order + notes + profile image URLs.
func (b *BasicService) GetAccounts(platformKey string) ([]AccountDTO, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return nil, nil
	}
	ids, err := readIDs(platformKey)
	if err != nil {
		return nil, err
	}
	order, err := readOrder(platformKey)
	if err != nil {
		return nil, err
	}
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	var keys []string
	for _, id := range order {
		if _, ok := ids[id]; ok {
			keys = append(keys, id)
			seen[id] = struct{}{}
		}
	}
	for id := range ids {
		if _, ok := seen[id]; !ok {
			keys = append(keys, id)
		}
	}

	out := make([]AccountDTO, 0, len(keys))
	for _, uid := range keys {
		name := ids[uid]
		note := ""
		if ps.AccountNotes != nil {
			note = ps.AccountNotes[uid]
		}
		img := ""
		if u, ok := profileimage.FindCached(platformKey, uid); ok {
			img = u
		}
		out = append(out, AccountDTO{
			PlatformKey: platformKey,
			UniqueID:    uid,
			DisplayName: name,
			ImageURL:    img,
			Note:        note,
		})
	}
	return out, nil
}

// SwapToAccount switches to a saved account (non-Steam).
func (b *BasicService) SwapToAccount(platformKey, uniqueID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return SwapTo(b.deps(), strings.TrimSpace(platformKey), strings.TrimSpace(uniqueID))
}

// SaveCurrent saves the live session to LoginCache under the given display name.
func (b *BasicService) SaveCurrent(platformKey, accountName string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return SaveCurrent(b.deps(), strings.TrimSpace(platformKey), strings.TrimSpace(accountName))
}

// AddNew clears login state and launches the platform exe.
func (b *BasicService) AddNew(platformKey string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return AddNew(b.deps(), strings.TrimSpace(platformKey))
}

// LaunchPlatform starts the platform without switching (delegates Steam in main).
func (b *BasicService) LaunchPlatform(platformKey string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return LaunchBasic(b.deps(), strings.TrimSpace(platformKey))
}

// ForgetAccount removes cached data for an account.
func (b *BasicService) ForgetAccount(platformKey, uniqueID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	platformKey = strings.TrimSpace(platformKey)
	uniqueID = strings.TrimSpace(uniqueID)
	ids, err := readIDs(platformKey)
	if err != nil {
		return err
	}
	name := ids[uniqueID]
	delete(ids, uniqueID)
	if err := writeIDs(platformKey, ids); err != nil {
		return err
	}
	if name != "" {
		dir, err := accountCacheDir(platformKey, name)
		if err == nil {
			_ = os.RemoveAll(dir)
		}
	}
	_ = profileimage.DeleteCached(platformKey, uniqueID)
	return nil
}

// SaveAccountOrder persists the order of unique ids for the platform.
func (b *BasicService) SaveAccountOrder(platformKey string, order []string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return writeOrder(strings.TrimSpace(platformKey), order)
}

// SetAccountNote updates AccountNotes in platform settings JSON.
func (b *BasicService) SetAccountNote(platformKey, uniqueID, note string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil {
		return err
	}
	if ps.AccountNotes == nil {
		ps.AccountNotes = map[string]string{}
	}
	uniqueID = strings.TrimSpace(uniqueID)
	if uniqueID == "" {
		return nil
	}
	ps.AccountNotes[uniqueID] = note
	return platform.SavePlatformSettings(strings.TrimSpace(platformKey), ps)
}
