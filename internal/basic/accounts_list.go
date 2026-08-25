package basic

import (
	"log/slog"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/accountlist"
	"TcNo-Acc-Switcher/internal/parallel"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/profileimage"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/stats"
)

// AccountListItemDTO is the fast account list payload (ids, names, order, live session).
type AccountListItemDTO struct {
	PlatformKey     string `json:"platformKey"`
	UniqueID        string `json:"uniqueId"`
	DisplayName     string `json:"displayName"`
	CurrentSession  bool   `json:"currentSession"`
	SavedDataBroken bool   `json:"savedDataBroken"`
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
	SavedDataBroken    bool            `json:"savedDataBroken"`
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
	if pruneExpiredTagsInFile(&idf, time.Now().UTC()) {
		persistPrunedTags(platformKey, idf)
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
	keys := accountlist.OrderedIDs(ids, order)

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
	if err := security.RequireUnlocked(); err != nil {
		return nil, err
	}

	ctx, err := b.buildAccountListContext(platformKey)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, nil
	}

	// With saved-account encryption on, SavedDataBroken reads and AEAD-decrypts
	// that account's whole blob, which dwarfs everything else here and scales
	// with the account count, so the rows are built concurrently. Without it the
	// row is a few map lookups and the fan-out would cost more than it saves.
	out := make([]AccountListItemDTO, len(ctx.keys))
	blobs := security.NewAccountBlobValidator()
	defer blobs.Close()
	parallel.ForEachIndexWhen(blobs.Encrypted(), len(ctx.keys), func(i int) {
		uid := ctx.keys[i]
		out[i] = AccountListItemDTO{
			PlatformKey:     ctx.platformKey,
			UniqueID:        uid,
			DisplayName:     ctx.ids[uid],
			CurrentSession:  ctx.liveUID != "" && strings.EqualFold(ctx.liveUID, uid),
			SavedDataBroken: !blobs.Valid(ctx.platformKey, uid),
		}
	})
	if len(out) > 0 {
		syncBasicPlatformCounts(ctx.platformKey, len(out), ctx.ps)
	}
	return out, nil
}

func (b *BasicService) GetAccountsEnrichment(platformKey string) ([]AccountEnrichmentDTO, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	defer closeSharedLevelDBHandles("GetAccountsEnrichment.end")
	if err := security.RequireUnlocked(); err != nil {
		return nil, err
	}

	ctx, err := b.buildAccountListContext(platformKey)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, nil
	}

	// One directory read answers every account's avatar and manual-marker
	// question. The per-account lookups cost about fifteen syscalls each, which
	// dominates this call once an install has more than a handful of accounts.
	avatars, err := profileimage.NewSnapshot(ctx.platformKey)
	if err != nil {
		return nil, err
	}

	// Same as GetAccountsList: worth fanning out exactly when SavedDataBroken
	// costs a blob decrypt per account.
	out := make([]AccountEnrichmentDTO, len(ctx.keys))
	blobs := security.NewAccountBlobValidator()
	defer blobs.Close()
	parallel.ForEachIndexWhen(blobs.Encrypted(), len(ctx.keys), func(i int) {
		uid := ctx.keys[i]
		note := ""
		if ctx.ps.AccountNotes != nil {
			note = ctx.ps.AccountNotes[uid]
		}
		img := ""
		pending := false
		if u, ok := avatars.FindCached(uid); ok {
			img = u
		}
		manual := avatars.HasManualProfileMarker(uid)
		if ctx.remoteProfilePics {
			if manual {
				pending = false
			} else {
				pending = avatars.OlderThanDays(uid, ctx.maxAge)
			}
		}
		lu := ""
		if ctx.lastUsedMap != nil {
			lu = strings.TrimSpace(ctx.lastUsedMap[uid])
		}
		out[i] = AccountEnrichmentDTO{
			UniqueID:           uid,
			ImageURL:           img,
			AvatarPending:      pending,
			ManualProfileImage: manual,
			Note:               note,
			LastUsed:           lu,
			ShowLastUsed:       ctx.ps.ShowLastUsed,
			Tags:               resolveTagsForAccount(ctx.idf, uid),
			SavedDataBroken:    !blobs.Valid(ctx.platformKey, uid),
		}
	})
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
		SavedDataBroken:    list.SavedDataBroken || enrich.SavedDataBroken,
	}
}

func syncBasicPlatformCounts(platformKey string, accountCount int, ps platform.PlatformSettings) {
	sc, hot := accountlist.ShortcutCounts(ps.Shortcuts)
	_ = stats.SyncPlatformCounts(platformKey, accountCount, sc, hot)
}
