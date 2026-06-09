package basic

import (
	"log/slog"
	"sort"
	"strings"

	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/profileimage"
	"TcNo-Acc-Switcher/internal/stats"
)

// AccountListItemDTO is the fast account list payload (ids, names, order, live session).
type AccountListItemDTO struct {
	PlatformKey    string `json:"platformKey"`
	UniqueID       string `json:"uniqueId"`
	DisplayName    string `json:"displayName"`
	CurrentSession bool   `json:"currentSession"`
}

// AccountEnrichmentDTO carries slower per-account metadata loaded after the list is shown.
type AccountEnrichmentDTO struct {
	UniqueID           string          `json:"uniqueId"`
	ImageURL           string          `json:"imageUrl"`
	AvatarPending      bool            `json:"avatarPending"`
	ManualProfileImage bool            `json:"manualProfileImage"`
	Note               string          `json:"note"`
	LastUsed           string          `json:"lastUsed"`
	ShowLastUsed       bool            `json:"showLastUsed"`
	Tags               []AccountTagDTO `json:"tags"`
}

type accountListContext struct {
	platformKey       string
	ids               map[string]string
	lastUsedMap       map[string]string
	keys              []string
	liveUID           string
	remoteProfilePics bool
	maxAge            int
	ps                platform.PlatformSettings
	idf               idsFile
}

func (b *BasicService) buildAccountListContext(platformKey string) (*accountListContext, error) {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return nil, nil
	}
	idf, err := readIdsFile(platformKey)
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

	ids := idf.IDs
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
	sort.Strings(missing)
	keys = append(keys, missing...)

	return &accountListContext{
		platformKey:       platformKey,
		ids:               ids,
		lastUsedMap:       idf.LastUsed,
		keys:              keys,
		liveUID:           liveUID,
		remoteProfilePics: remoteProfilePics,
		maxAge:            maxAge,
		ps:                ps,
		idf:               idf,
	}, nil
}

func (b *BasicService) GetAccountsList(platformKey string) ([]AccountListItemDTO, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	defer closeSharedLevelDBHandles("GetAccountsList.end")

	ctx, err := b.buildAccountListContext(platformKey)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, nil
	}

	out := make([]AccountListItemDTO, 0, len(ctx.keys))
	for _, uid := range ctx.keys {
		out = append(out, AccountListItemDTO{
			PlatformKey:    ctx.platformKey,
			UniqueID:       uid,
			DisplayName:    ctx.ids[uid],
			CurrentSession: ctx.liveUID != "" && strings.EqualFold(ctx.liveUID, uid),
		})
	}
	if len(out) > 0 {
		syncBasicPlatformCounts(ctx.platformKey, len(out), ctx.ps)
	}
	return out, nil
}

func (b *BasicService) GetAccountsEnrichment(platformKey string) ([]AccountEnrichmentDTO, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	defer closeSharedLevelDBHandles("GetAccountsEnrichment.end")

	ctx, err := b.buildAccountListContext(platformKey)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, nil
	}

	out := make([]AccountEnrichmentDTO, 0, len(ctx.keys))
	for _, uid := range ctx.keys {
		note := ""
		if ctx.ps.AccountNotes != nil {
			note = ctx.ps.AccountNotes[uid]
		}
		img := ""
		pending := false
		if u, ok := profileimage.FindCached(ctx.platformKey, uid); ok {
			img = u
		}
		if ctx.remoteProfilePics {
			if profileimage.HasManualProfileMarker(ctx.platformKey, uid) {
				pending = false
			} else if p, ok := profileimage.CachedFilePath(ctx.platformKey, uid); ok {
				pending = profileimage.FileOlderThanDays(p, ctx.maxAge)
			} else {
				pending = true
			}
		}
		lu := ""
		if ctx.lastUsedMap != nil {
			lu = strings.TrimSpace(ctx.lastUsedMap[uid])
		}
		out = append(out, AccountEnrichmentDTO{
			UniqueID:           uid,
			ImageURL:           img,
			AvatarPending:      pending,
			ManualProfileImage: profileimage.HasManualProfileMarker(ctx.platformKey, uid),
			Note:               note,
			LastUsed:           lu,
			ShowLastUsed:       ctx.ps.ShowLastUsed,
			Tags:               resolveTagsForAccount(ctx.idf, uid),
		})
	}
	return out, nil
}

func mergeBasicAccountDTO(list AccountListItemDTO, enrich AccountEnrichmentDTO) AccountDTO {
	return AccountDTO{
		PlatformKey:        list.PlatformKey,
		UniqueID:           list.UniqueID,
		DisplayName:        list.DisplayName,
		CurrentSession:     list.CurrentSession,
		ImageURL:           enrich.ImageURL,
		AvatarPending:      enrich.AvatarPending,
		ManualProfileImage: enrich.ManualProfileImage,
		Note:               enrich.Note,
		LastUsed:           enrich.LastUsed,
		ShowLastUsed:       enrich.ShowLastUsed,
		Tags:               enrich.Tags,
	}
}

func syncBasicPlatformCounts(platformKey string, accountCount int, ps platform.PlatformSettings) {
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
	_ = stats.SyncPlatformCounts(platformKey, accountCount, sc, hot)
}
