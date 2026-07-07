package basic

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/accountlist"
	"TcNo-Acc-Switcher/internal/paths"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/profileimage"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/stats"
	"TcNo-Acc-Switcher/internal/tray"
	"TcNo-Acc-Switcher/internal/winutil"
)

type BasicService struct {
	mu                      sync.Mutex
	PS                      *platform.PlatformService
	gameStatsActiveMu       sync.RWMutex
	gameStatsActivePlatform string

	imgRefreshCooldown  *perPlatformCooldown
	imgRefreshCoalescer *perPlatformCoalescer
	imgRefreshClock     func() time.Time
}

// imgRefreshMinInterval is the per-platform cooldown between background
// profile-image refresh scans. State is in-memory only and resets on process
// restart.
const imgRefreshMinInterval = 30 * time.Second

func NewBasicService(ps *platform.PlatformService) *BasicService {
	return &BasicService{
		PS:                  ps,
		imgRefreshCooldown:  newPerPlatformCooldown(imgRefreshMinInterval, time.Now),
		imgRefreshCoalescer: newPerPlatformCoalescer(),
		imgRefreshClock:     time.Now,
	}
}

func (b *BasicService) deps() FlowDeps {
	return FlowDeps{PS: b.PS}
}

type AccountDTO struct {
	PlatformKey        string          `json:"platformKey"`
	UniqueID           string          `json:"uniqueId"`
	DisplayName        string          `json:"displayName"`
	ImageURL           string          `json:"imageUrl"`
	AvatarPending      bool            `json:"avatarPending"`
	ManualProfileImage bool            `json:"manualProfileImage"`
	Note               string          `json:"note"`
	CurrentSession     bool            `json:"currentSession"`
	LastUsed           string          `json:"lastUsed"`
	ShowLastUsed       bool            `json:"showLastUsed"`
	Tags               []AccountTagDTO `json:"tags"`
	SavedDataBroken    bool            `json:"savedDataBroken"`
}

func (b *BasicService) GetAccounts(platformKey string) ([]AccountDTO, error) {
	if err := security.RequireUnlocked(); err != nil {
		return nil, err
	}
	list, err := b.GetAccountsList(platformKey)
	if err != nil {
		return nil, err
	}
	enrich, err := b.GetAccountsEnrichment(platformKey)
	if err != nil {
		return nil, err
	}
	out := accountlist.Merge(
		list,
		enrich,
		func(row AccountListItemDTO) string { return row.UniqueID },
		func(row AccountEnrichmentDTO) string { return row.UniqueID },
		mergeBasicAccountDTO,
	)
	if len(out) > 0 {
		ps, perr := platform.LoadPlatformSettings(platformKey)
		if perr == nil {
			syncBasicPlatformCounts(platformKey, len(out), ps)
		}
	}
	return out, nil
}

func (b *BasicService) SwapToAccount(platformKey, uniqueID string, extraLaunchArgs []string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	return SwapTo(b.deps(), strings.TrimSpace(platformKey), strings.TrimSpace(uniqueID), extraLaunchArgs)
}

func (b *BasicService) SaveCurrent(platformKey, accountName string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
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
	if err := security.RequireUnlocked(); err != nil {
		return "", err
	}
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
			_ = winutil.KillByName(d.ExesToEnd, winutil.ClosingMethod(ps.ClosingMethod), electronBeforeKillSynth(b.deps(), platformKey, d.ExesToEnd))
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
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	return AddNew(b.deps(), strings.TrimSpace(platformKey))
}

func (b *BasicService) LaunchPlatform(platformKey string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	return LaunchBasic(b.deps(), strings.TrimSpace(platformKey), nil)
}

func (b *BasicService) ForgetAccount(platformKey, uniqueID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
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
	delete(f.AccountTagExpiries, uniqueID)
	pruneUnusedTagDefinitions(&f)
	if err := writeIdsFile(platformKey, f); err != nil {
		return err
	}
	syncBasicTrayKnownAccounts(platformKey, f.IDs)
	tray.RefreshMenuIfSet()
	if name != "" {
		dir, err := accountCacheDir(platformKey, name)
		if err == nil {
			_ = security.RemoveAccountCache(platformKey, uniqueID, name, dir)
		}
	}
	_ = profileimage.DeleteCached(platformKey, uniqueID)
	return nil
}

func (b *BasicService) SaveAccountOrder(platformKey string, order []string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	return writeOrder(strings.TrimSpace(platformKey), order)
}

func (b *BasicService) SetAccountNote(platformKey, uniqueID, note string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
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
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
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
	if !security.SavedAccountDataEncrypted() && strings.TrimSpace(oldName) != "" {
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
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	platformKey = strings.TrimSpace(platformKey)
	uniqueID = strings.TrimSpace(uniqueID)
	sourcePath = strings.TrimSpace(sourcePath)
	if platformKey == "" || uniqueID == "" || sourcePath == "" {
		return fmt.Errorf("invalid change image parameters")
	}
	// Lock manual marker before writing bytes so concurrent basic profile refresh cannot download/copy
	// over the avatar during the gap where DeleteCached (old behavior) stripped the sentinel (Rockstar/EA/Ubisoft).
	if err := profileimage.WriteManualProfileMarker(platformKey, uniqueID); err != nil {
		return err
	}
	if err := profileimage.CacheLocalFileForUser(platformKey, uniqueID, sourcePath); err != nil {
		_ = profileimage.ClearManualProfileMarker(platformKey, uniqueID)
		return err
	}
	return nil
}

// ClearManualAccountProfileImage removes a user-set avatar and allows automated images again for this account.
func (b *BasicService) ClearManualAccountProfileImage(platformKey, uniqueID string) error {
	platformKey = strings.TrimSpace(platformKey)
	uniqueID = strings.TrimSpace(uniqueID)
	b.mu.Lock()
	if err := security.RequireUnlocked(); err != nil {
		b.mu.Unlock()
		return err
	}
	err := profileimage.DeleteCached(platformKey, uniqueID)
	var accountName string
	if err == nil {
		if f, ferr := readIdsFile(platformKey); ferr == nil {
			accountName = strings.TrimSpace(f.IDs[uniqueID])
		}
	}
	d, _, derr := readDescriptor(platformKey)
	folder, _ := resolveExeFolder(b.deps(), platformKey)
	b.mu.Unlock()
	if err != nil {
		return err
	}
	if derr == nil {
		_ = queueAutomatedProfileImage(platformKey, uniqueID, accountName, d, folder)
	}
	b.StartBasicProfileImageRefresh(platformKey)
	return nil
}

func (b *BasicService) GetAccountNote(platformKey, uniqueID string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return "", err
	}
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
	if err := security.RequireUnlocked(); err != nil {
		return nil, err
	}
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
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
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
		clearAccountTagExpiry(&f, uniqueID, tagID)
		if err := writeIdsFile(platformKey, f); err != nil {
			return err
		}
		_ = stats.SyncPlatformTagCounts(platformKey, len(f.Tags), countTaggedAccounts(f))
		return nil
	}
	f.AccountTags[uniqueID] = append(cur, tagID)
	clearAccountTagExpiry(&f, uniqueID, tagID)
	if err := writeIdsFile(platformKey, f); err != nil {
		return err
	}
	_ = stats.SyncPlatformTagCounts(platformKey, len(f.Tags), countTaggedAccounts(f))
	return nil
}

func (b *BasicService) AddTagToAccounts(platformKey string, uniqueIDs []string, name string) (TagDefinitionDTO, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var zero TagDefinitionDTO
	if err := security.RequireUnlocked(); err != nil {
		return zero, err
	}
	platformKey = strings.TrimSpace(platformKey)
	name, err := tagNameOK(name)
	if err != nil {
		return zero, err
	}
	if platformKey == "" {
		return zero, fmt.Errorf("invalid tag parameters")
	}
	f, err := readIdsFile(platformKey)
	if err != nil {
		return zero, err
	}
	normalizeTagMaps(&f)
	tagID := ""
	tag := tagFileEntry{}
	for id, def := range f.Tags {
		if strings.EqualFold(strings.TrimSpace(def.Name), name) {
			tagID = id
			tag = def
			break
		}
	}
	if tagID == "" {
		tagID, err = newTagID()
		if err != nil {
			return zero, err
		}
		color, err := randomSaturatedColorHex()
		if err != nil {
			return zero, err
		}
		tag = tagFileEntry{Name: name, Color: color}
		f.Tags[tagID] = tag
	}
	addedToAny := false
	seen := map[string]struct{}{}
	for _, uid := range uniqueIDs {
		uid = strings.TrimSpace(uid)
		if uid == "" {
			continue
		}
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		if _, ok := f.IDs[uid]; !ok {
			continue
		}
		cur := f.AccountTags[uid]
		if containsTagID(cur, tagID) {
			clearAccountTagExpiry(&f, uid, tagID)
			addedToAny = true
			continue
		}
		f.AccountTags[uid] = append(cur, tagID)
		clearAccountTagExpiry(&f, uid, tagID)
		addedToAny = true
	}
	if !addedToAny {
		if len(uniqueIDs) == 0 {
			delete(f.Tags, tagID)
		}
		return zero, fmt.Errorf("no matching accounts")
	}
	if err := writeIdsFile(platformKey, f); err != nil {
		return zero, err
	}
	_ = stats.SyncPlatformTagCounts(platformKey, len(f.Tags), countTaggedAccounts(f))
	return TagDefinitionDTO{
		ID:        tagID,
		Name:      strings.TrimSpace(tag.Name),
		Color:     strings.TrimSpace(tag.Color),
		ExpiresAt: strings.TrimSpace(tag.ExpiresAt),
	}, nil
}

func (b *BasicService) ClearAccountTags(platformKey string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return fmt.Errorf("invalid tag parameters")
	}
	f, err := readIdsFile(platformKey)
	if err != nil {
		return err
	}
	normalizeTagMaps(&f)
	f.AccountTags = map[string][]string{}
	f.AccountTagExpiries = map[string]map[string]string{}
	pruneUnusedTagDefinitions(&f)
	if err := writeIdsFile(platformKey, f); err != nil {
		return err
	}
	_ = stats.SyncPlatformTagCounts(platformKey, len(f.Tags), countTaggedAccounts(f))
	return nil
}

func (b *BasicService) RemoveTagFromAccount(platformKey, uniqueID, tagID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
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
	clearAccountTagExpiry(&f, uniqueID, tagID)
	pruneUnusedTagDefinitions(&f)
	if err := writeIdsFile(platformKey, f); err != nil {
		return err
	}
	_ = stats.SyncPlatformTagCounts(platformKey, len(f.Tags), countTaggedAccounts(f))
	return nil
}

func (b *BasicService) CreateTagAndAddToAccount(platformKey, uniqueID, name string) (TagDefinitionDTO, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var zero TagDefinitionDTO
	if err := security.RequireUnlocked(); err != nil {
		return zero, err
	}
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
	_ = stats.SyncPlatformTagCounts(platformKey, len(f.Tags), countTaggedAccounts(f))
	return TagDefinitionDTO{ID: id, Name: name, Color: color}, nil
}

func (b *BasicService) RemoveTagFromAllAccounts(platformKey, tagID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	platformKey = strings.TrimSpace(platformKey)
	tagID = strings.TrimSpace(tagID)
	if platformKey == "" || tagID == "" {
		return fmt.Errorf("invalid tag parameters")
	}
	f, err := readIdsFile(platformKey)
	if err != nil {
		return err
	}
	normalizeTagMaps(&f)
	for uid, cur := range f.AccountTags {
		next := cur[:0]
		for _, id := range cur {
			if strings.EqualFold(strings.TrimSpace(id), tagID) {
				continue
			}
			next = append(next, id)
		}
		if len(next) == 0 {
			delete(f.AccountTags, uid)
		} else {
			f.AccountTags[uid] = next
		}
		clearAccountTagExpiry(&f, uid, tagID)
	}
	pruneUnusedTagDefinitions(&f)
	if err := writeIdsFile(platformKey, f); err != nil {
		return err
	}
	_ = stats.SyncPlatformTagCounts(platformKey, len(f.Tags), countTaggedAccounts(f))
	return nil
}

func (b *BasicService) SetTagExpiry(platformKey, uniqueID, tagID, scope, expiresAt string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return fmt.Errorf("invalid tag parameters")
	}
	f, err := readIdsFile(platformKey)
	if err != nil {
		return err
	}
	if err := setTagExpiryOnFile(&f, uniqueID, tagID, scope, expiresAt); err != nil {
		return err
	}
	if err := writeIdsFile(platformKey, f); err != nil {
		return err
	}
	return nil
}

func (b *BasicService) PruneExpiredTags(platformKey string) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return false, err
	}
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return false, fmt.Errorf("invalid tag parameters")
	}
	f, err := readIdsFile(platformKey)
	if err != nil {
		return false, err
	}
	if !pruneExpiredTagsInFile(&f, time.Now().UTC()) {
		return false, nil
	}
	if err := writeIdsFile(platformKey, f); err != nil {
		return false, err
	}
	_ = stats.SyncPlatformTagCounts(platformKey, len(f.Tags), countTaggedAccounts(f))
	return true, nil
}

func (b *BasicService) ApplySpecialTag(platformKey, uniqueID, specialID string) (TagDefinitionDTO, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var zero TagDefinitionDTO
	if err := security.RequireUnlocked(); err != nil {
		return zero, err
	}
	platformKey = strings.TrimSpace(platformKey)
	uniqueID = strings.TrimSpace(uniqueID)
	specialID = strings.TrimSpace(specialID)
	if platformKey == "" || uniqueID == "" {
		return zero, fmt.Errorf("invalid tag parameters")
	}
	if specialID != "cs2-drop-claimed" {
		return zero, fmt.Errorf("unknown special tag")
	}
	f, err := readIdsFile(platformKey)
	if err != nil {
		return zero, err
	}
	normalizeTagMaps(&f)
	tagName := "CS2 Drop Claimed"
	tagID := ""
	tag := tagFileEntry{}
	for id, def := range f.Tags {
		if strings.EqualFold(strings.TrimSpace(def.Name), tagName) {
			tagID = id
			tag = def
			break
		}
	}
	if tagID == "" {
		tagID, err = newTagID()
		if err != nil {
			return zero, err
		}
		color, err := randomSaturatedColorHex()
		if err != nil {
			return zero, err
		}
		tag = tagFileEntry{Name: tagName, Color: color}
	}
	tag.ExpiresAt = nextCS2DropReset(time.Now().UTC()).Format(time.RFC3339)
	f.Tags[tagID] = tag
	cur := f.AccountTags[uniqueID]
	if !containsTagID(cur, tagID) {
		f.AccountTags[uniqueID] = append(cur, tagID)
	}
	clearAccountTagExpiry(&f, uniqueID, tagID)
	if err := writeIdsFile(platformKey, f); err != nil {
		return zero, err
	}
	_ = stats.SyncPlatformTagCounts(platformKey, len(f.Tags), countTaggedAccounts(f))
	return TagDefinitionDTO{
		ID:        tagID,
		Name:      strings.TrimSpace(tag.Name),
		Color:     strings.TrimSpace(tag.Color),
		ExpiresAt: strings.TrimSpace(tag.ExpiresAt),
	}, nil
}
