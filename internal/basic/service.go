package basic

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"

	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/profileimage"
	"TcNo-Acc-Switcher/internal/stats"
	"TcNo-Acc-Switcher/internal/winutil"
)

type BasicService struct {
	mu sync.Mutex
	PS *platform.PlatformService

	imgRefreshMu      sync.Mutex
	imgRefreshRunning bool
	imgRefreshQueued  bool
}

func NewBasicService(ps *platform.PlatformService) *BasicService {
	return &BasicService{PS: ps}
}

func (b *BasicService) deps() FlowDeps {
	return FlowDeps{PS: b.PS}
}

type AccountDTO struct {
	PlatformKey    string          `json:"platformKey"`
	UniqueID       string          `json:"uniqueId"`
	DisplayName    string          `json:"displayName"`
	ImageURL       string          `json:"imageUrl"`
	AvatarPending  bool            `json:"avatarPending"`
	Note           string          `json:"note"`
	CurrentSession bool            `json:"currentSession"`
	LastUsed       string          `json:"lastUsed"`
	ShowLastUsed   bool            `json:"showLastUsed"`
	Tags           []AccountTagDTO `json:"tags"`
}

func (b *BasicService) GetAccounts(platformKey string) ([]AccountDTO, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	defer closeSharedLevelDBHandles("GetAccounts.end")
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return nil, nil
	}
	idf, err := readIdsFile(platformKey)
	if err != nil {
		return nil, err
	}
	ids := idf.IDs
	lastUsedMap := idf.LastUsed
	order, err := readOrder(platformKey)
	if err != nil {
		return nil, err
	}
	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil {
		return nil, err
	}

	liveUID := ""
	remoteProfilePics := false
	if d, _, derr := readDescriptor(platformKey); derr == nil {
		folder, _ := resolveExeFolder(b.deps(), platformKey)
		if u, uerr := ReadUniqueID(platformKey, d, folder); uerr == nil {
			liveUID = strings.TrimSpace(u)
		} else {
			slog.Debug("list accounts: live unique id read failed", "platform", platformKey, "method", d.UniqueIdMethod, "file", d.UniqueIdFile, "err", uerr)
		}
		tpl := strings.TrimSpace(d.Extras.ProfilePicPath)
		remoteProfilePics = remoteProfilePicTemplate(tpl) && !strings.Contains(tpl, "%LARGEST%")
	}
	maxAge := ps.ProfileImageExpiryDays
	if maxAge <= 0 {
		maxAge = 7
	}

	seen := map[string]struct{}{}
	var keys []string
	for _, id := range order {
		if _, ok := ids[id]; ok {
			keys = append(keys, id)
			seen[id] = struct{}{}
		}
	}
	var missing []string
	for id := range ids {
		if _, ok := seen[id]; !ok {
			missing = append(missing, id)
		}
	}
	// Keep fallback order stable when order file is stale/missing entries.
	sort.Strings(missing)
	keys = append(keys, missing...)

	out := make([]AccountDTO, 0, len(keys))
	for _, uid := range keys {
		name := ids[uid]
		note := ""
		if ps.AccountNotes != nil {
			note = ps.AccountNotes[uid]
		}
		img := ""
		pending := false
		if u, ok := profileimage.FindCached(platformKey, uid); ok {
			img = u
		}
		if remoteProfilePics {
			if p, ok := profileimage.CachedFilePath(platformKey, uid); ok {
				pending = profileimage.FileOlderThanDays(p, maxAge)
			} else {
				pending = true
			}
		}
		lu := ""
		if lastUsedMap != nil {
			lu = strings.TrimSpace(lastUsedMap[uid])
		}
		out = append(out, AccountDTO{
			PlatformKey:    platformKey,
			UniqueID:       uid,
			DisplayName:    name,
			ImageURL:       img,
			AvatarPending:  pending,
			Note:           note,
			CurrentSession: liveUID != "" && strings.EqualFold(liveUID, uid),
			LastUsed:       lu,
			ShowLastUsed:   ps.ShowLastUsed,
			Tags:           resolveTagsForAccount(idf, uid),
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
	platformKey = strings.TrimSpace(platformKey)
	accountName = strings.TrimSpace(accountName)
	err := SaveCurrent(b.deps(), platformKey, accountName)
	if err != nil {
		slog.Default().Error("SaveCurrent failed",
			"platform", platformKey,
			"account", accountName,
			"err", err)
	}
	return err
}

// SuggestedSaveAccountName returns a platform-specific suggested display name for Save Current.
func (b *BasicService) SuggestedSaveAccountName(platformKey string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	defer closeSharedLevelDBHandles("SuggestedSaveAccountName.end")
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		slog.Debug("suggested save name skipped: empty platform")
		return "", nil
	}
	if d, _, derr := readDescriptor(platformKey); derr == nil && d.ExitBeforeSave {
		if ps, perr := platform.LoadPlatformSettings(platformKey); perr == nil {
			if err := winutil.ErrIfCannotKill(d.ExesToEnd, winutil.ClosingMethod(ps.ClosingMethod)); err != nil {
				slog.Debug("suggested save name pre-kill blocked", "platform", platformKey, "err", err)
				return "", err
			}
			slog.Debug("suggested save name pre-kill", "platform", platformKey, "reason", "ExitBeforeSave")
			_ = winutil.KillByName(d.ExesToEnd, winutil.ClosingMethod(ps.ClosingMethod))
		}
	}
	folder, _ := resolveExeFolder(b.deps(), platformKey)
	slog.Debug("suggested save name collect begin", "platform", platformKey, "folder", folder)
	ctx := platform.PathTokenContext{PlatformFolder: folder}
	name, found, err := platformSuggestedSaveName(platformKey, folder, ctx)
	if err != nil {
		slog.Debug("suggested save name provider error", "platform", platformKey, "err", err)
		return "", err
	}
	if !found {
		slog.Debug("suggested save name provider missing", "platform", platformKey)
		return "", nil
	}
	slog.Debug("suggested save name collected", "platform", platformKey, "name", strings.TrimSpace(name))
	return strings.TrimSpace(name), nil
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
	f, err := readIdsFile(platformKey)
	if err != nil {
		return err
	}
	name := f.IDs[uniqueID]
	delete(f.IDs, uniqueID)
	if f.LastUsed != nil {
		delete(f.LastUsed, uniqueID)
	}
	normalizeTagMaps(&f)
	delete(f.AccountTags, uniqueID)
	pruneUnusedTagDefinitions(&f)
	if err := writeIdsFile(platformKey, f); err != nil {
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

func (b *BasicService) ListTagDefinitions(platformKey string) ([]TagDefinitionDTO, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return nil, nil
	}
	f, err := readIdsFile(platformKey)
	if err != nil {
		return nil, err
	}
	return listTagDefinitionsSorted(f), nil
}

func (b *BasicService) AddTagToAccount(platformKey, uniqueID, tagID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	platformKey = strings.TrimSpace(platformKey)
	uniqueID = strings.TrimSpace(uniqueID)
	tagID = strings.TrimSpace(tagID)
	if platformKey == "" || uniqueID == "" || tagID == "" {
		return fmt.Errorf("invalid tag parameters")
	}
	f, err := readIdsFile(platformKey)
	if err != nil {
		return err
	}
	normalizeTagMaps(&f)
	if _, ok := f.Tags[tagID]; !ok {
		return fmt.Errorf("unknown tag id")
	}
	cur := f.AccountTags[uniqueID]
	if containsTagID(cur, tagID) {
		return nil
	}
	f.AccountTags[uniqueID] = append(cur, tagID)
	return writeIdsFile(platformKey, f)
}

func (b *BasicService) RemoveTagFromAccount(platformKey, uniqueID, tagID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	platformKey = strings.TrimSpace(platformKey)
	uniqueID = strings.TrimSpace(uniqueID)
	tagID = strings.TrimSpace(tagID)
	if platformKey == "" || uniqueID == "" || tagID == "" {
		return fmt.Errorf("invalid tag parameters")
	}
	f, err := readIdsFile(platformKey)
	if err != nil {
		return err
	}
	normalizeTagMaps(&f)
	cur := f.AccountTags[uniqueID]
	if len(cur) == 0 {
		return nil
	}
	next := cur[:0]
	for _, x := range cur {
		if strings.EqualFold(strings.TrimSpace(x), tagID) {
			continue
		}
		next = append(next, x)
	}
	if len(next) == 0 {
		delete(f.AccountTags, uniqueID)
	} else {
		f.AccountTags[uniqueID] = next
	}
	pruneUnusedTagDefinitions(&f)
	return writeIdsFile(platformKey, f)
}

func (b *BasicService) CreateTagAndAddToAccount(platformKey, uniqueID, name string) (TagDefinitionDTO, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var zero TagDefinitionDTO
	platformKey = strings.TrimSpace(platformKey)
	uniqueID = strings.TrimSpace(uniqueID)
	name, err := tagNameOK(name)
	if err != nil {
		return zero, err
	}
	if platformKey == "" || uniqueID == "" {
		return zero, fmt.Errorf("invalid tag parameters")
	}
	f, err := readIdsFile(platformKey)
	if err != nil {
		return zero, err
	}
	normalizeTagMaps(&f)
	id, err := newTagID()
	if err != nil {
		return zero, err
	}
	color, err := randomSaturatedColorHex()
	if err != nil {
		return zero, err
	}
	f.Tags[id] = tagFileEntry{Name: name, Color: color}
	cur := f.AccountTags[uniqueID]
	if !containsTagID(cur, id) {
		f.AccountTags[uniqueID] = append(cur, id)
	}
	if err := writeIdsFile(platformKey, f); err != nil {
		return zero, err
	}
	return TagDefinitionDTO{ID: id, Name: name, Color: color}, nil
}
