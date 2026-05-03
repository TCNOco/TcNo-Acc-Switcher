package basic

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/profileimage"
	"TcNo-Acc-Switcher/internal/stats"
)

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

type AccountDTO struct {
	PlatformKey    string `json:"platformKey"`
	UniqueID       string `json:"uniqueId"`
	DisplayName    string `json:"displayName"`
	ImageURL       string `json:"imageUrl"`
	Note           string `json:"note"`
	CurrentSession bool   `json:"currentSession"`
}

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

	liveUID := ""
	if d, _, derr := readDescriptor(platformKey); derr == nil {
		folder, _ := resolveExeFolder(b.deps(), platformKey)
		if u, uerr := ReadUniqueID(d, folder); uerr == nil {
			liveUID = strings.TrimSpace(u)
		}
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
			PlatformKey:    platformKey,
			UniqueID:       uid,
			DisplayName:    name,
			ImageURL:       img,
			Note:           note,
			CurrentSession: liveUID != "" && strings.EqualFold(liveUID, uid),
		})
	}
	sc, hot := 0, 0
	for _, e := range ps.Shortcuts {
		fn := strings.TrimSpace(e.FileName)
		if fn == "" {
			continue
		}
		sc++
		if e.Pinned {
			hot++
		}
	}
	_ = stats.SyncPlatformCounts(platformKey, len(out), sc, hot)
	return out, nil
}

func (b *BasicService) SwapToAccount(platformKey, uniqueID string, extraLaunchArgs []string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return SwapTo(b.deps(), strings.TrimSpace(platformKey), strings.TrimSpace(uniqueID), extraLaunchArgs)
}

func (b *BasicService) SaveCurrent(platformKey, accountName string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return SaveCurrent(b.deps(), strings.TrimSpace(platformKey), strings.TrimSpace(accountName))
}

func (b *BasicService) AddNew(platformKey string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return AddNew(b.deps(), strings.TrimSpace(platformKey))
}

func (b *BasicService) LaunchPlatform(platformKey string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return LaunchBasic(b.deps(), strings.TrimSpace(platformKey), nil)
}

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

func (b *BasicService) SaveAccountOrder(platformKey string, order []string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return writeOrder(strings.TrimSpace(platformKey), order)
}

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

func (b *BasicService) RenameAccount(platformKey, uniqueID, newName string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	platformKey = strings.TrimSpace(platformKey)
	uniqueID = strings.TrimSpace(uniqueID)
	newName = paths.WindowsFileName(strings.TrimSpace(newName), 200)
	if platformKey == "" || uniqueID == "" || newName == "" {
		return fmt.Errorf("invalid rename parameters")
	}
	ids, err := readIDs(platformKey)
	if err != nil {
		return err
	}
	oldName, ok := ids[uniqueID]
	if !ok {
		return fmt.Errorf("unknown account")
	}
	if oldName == newName {
		return nil
	}
	ids[uniqueID] = newName
	if err := writeIDs(platformKey, ids); err != nil {
		return err
	}
	if strings.TrimSpace(oldName) != "" {
		oldDir, err := accountCacheDir(platformKey, oldName)
		if err == nil {
			newDir, err2 := accountCacheDir(platformKey, newName)
			if err2 == nil && oldDir != newDir {
				_ = os.Rename(oldDir, newDir)
			}
		}
	}
	return nil
}

func (b *BasicService) ChangeAccountImage(platformKey, uniqueID, sourcePath string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return profileimage.CacheLocalFile(strings.TrimSpace(platformKey), strings.TrimSpace(uniqueID), strings.TrimSpace(sourcePath))
}

func (b *BasicService) GetAccountNote(platformKey, uniqueID string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ps, err := platform.LoadPlatformSettings(strings.TrimSpace(platformKey))
	if err != nil {
		return "", err
	}
	if ps.AccountNotes == nil {
		return "", nil
	}
	return ps.AccountNotes[strings.TrimSpace(uniqueID)], nil
}
