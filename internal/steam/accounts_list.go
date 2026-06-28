package steam

import (
	"fmt"
	"log/slog"
	"strings"

	"TcNo-Acc-Switcher/internal/accountlist"
	"TcNo-Acc-Switcher/internal/basic"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/profileimage"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/stats"
)

// SteamAccountListItemDTO is the fast Steam account list payload.
type SteamAccountListItemDTO struct {
	SteamID64      string `json:"steamId64"`
	PersonaName    string `json:"personaName"`
	DisplayName    string `json:"displayName"`
	AccountName    string `json:"accountName"`
	CurrentSession bool   `json:"currentSession"`
}

// SteamAccountEnrichmentDTO carries slower per-account Steam metadata.
type SteamAccountEnrichmentDTO struct {
	SteamID64 string `json:"steamId64"`

	DisplayName        string                `json:"displayName"`
	LastLogin          string                `json:"lastLogin"`
	Offline            bool                  `json:"offline"`
	ImageURL           string                `json:"imageUrl"`
	StaticImageURL     string                `json:"staticImageUrl"`
	AvatarPending      bool                  `json:"avatarPending"`
	MetaPending        bool                  `json:"metaPending"`
	Vac                bool                  `json:"vac"`
	Ltd                bool                  `json:"ltd"`
	ShowSteamID        bool                  `json:"showSteamId"`
	ShowVAC            bool                  `json:"showVac"`
	ShowLimited        bool                  `json:"showLimited"`
	ShowLastLogin      bool                  `json:"showLastLogin"`
	ShowAccUsername    bool                  `json:"showAccUsername"`
	CollectInfo        bool                  `json:"collectInfo"`
	ShowShortNotes     bool                  `json:"showShortNotes"`
	Note               string                `json:"note"`
	AvatarFrameURL     string                `json:"avatarFrameUrl"`
	MiniProfileHTML    string                `json:"miniProfileHtml"`
	ShowMiniProfile    bool                  `json:"showMiniProfile"`
	ShowAvatarFrame    bool                  `json:"showAvatarFrame"`
	SyncError          string                `json:"syncError"`
	Tags               []basic.AccountTagDTO `json:"tags"`
	ManualProfileImage bool                  `json:"manualProfileImage"`
}

type steamListContext struct {
	users            []LoginUser
	activeSteamID    string
	st               Settings
	app              platform.AppSettings
	effectiveCollect bool
	vacMap           map[string]VacEntry
	vacKnown         map[string]struct{}
	tagByUID         map[string][]basic.AccountTagDTO
}

func (s *SteamService) buildSteamListContext() (*steamListContext, error) {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return nil, err
	}
	app, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		return nil, err
	}
	st, err := LoadSettings()
	if err != nil {
		return nil, err
	}
	_ = s.migrateExePathFromAppSettings(exeDir, &st, &app)

	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		return nil, err
	}

	root, err := ResolveInstallFolder(exeDir, st, app, raw)
	if err != nil {
		steamLog.Error("ResolveInstallFolder failed", slog.Any("err", err))
		return nil, err
	}
	if root == "" {
		steamLog.Error("steam install folder not found after resolution")
		return nil, fmt.Errorf("steam install folder not found")
	}

	loginPath := LoginUsersPath(root)
	users, err := ParseLoginUsers(loginPath)
	if err != nil {
		steamLog.Error("ParseLoginUsers failed", slog.String("path", loginPath), slog.Any("err", err))
		return nil, err
	}

	order, err := LoadOrder()
	if err != nil {
		return nil, err
	}
	users = MergeOrder(order, users)
	activeSteamID := ActiveSessionSteamID64(users)

	vacRows, err := LoadVacCache(st.SteamImageExpiryTime)
	if err != nil {
		return nil, err
	}
	vm := vacMap(vacRows)
	vacKnown := make(map[string]struct{}, len(vacRows))
	for _, r := range vacRows {
		if r.SteamID != "" {
			vacKnown[r.SteamID] = struct{}{}
		}
	}

	tagByUID, _ := basic.BuildAccountTagMap(PlatformKey)

	return &steamListContext{
		users:            users,
		activeSteamID:    activeSteamID,
		st:               st,
		app:              app,
		effectiveCollect: st.CollectInfo && !app.OfflineMode,
		vacMap:           vm,
		vacKnown:         vacKnown,
		tagByUID:         tagByUID,
	}, nil
}

func (s *SteamService) GetSteamAccountsList() ([]SteamAccountListItemDTO, error) {
	if err := security.RequireUnlocked(); err != nil {
		return nil, err
	}
	ctx, err := s.buildSteamListContext()
	if err != nil {
		return nil, err
	}

	out := make([]SteamAccountListItemDTO, 0, len(ctx.users))
	for _, u := range ctx.users {
		persona := displayPersona(u)
		out = append(out, SteamAccountListItemDTO{
			SteamID64:      u.SteamID64,
			PersonaName:    persona,
			DisplayName:    persona,
			AccountName:    strings.TrimSpace(u.AccountName),
			CurrentSession: ctx.activeSteamID != "" && u.SteamID64 == ctx.activeSteamID,
		})
	}
	if len(out) > 0 {
		syncSteamPlatformCounts(len(out))
	}
	return out, nil
}

func (s *SteamService) GetSteamAccountsEnrichment() ([]SteamAccountEnrichmentDTO, error) {
	if err := security.RequireUnlocked(); err != nil {
		return nil, err
	}
	ctx, err := s.buildSteamListContext()
	if err != nil {
		return nil, err
	}

	out := make([]SteamAccountEnrichmentDTO, 0, len(ctx.users))
	for _, u := range ctx.users {
		v := ctx.vacMap[u.SteamID64]
		primaryURL, _ := profileimage.FindCached(PlatformKey, u.SteamID64)
		staticURL, _ := profileimage.FindCached(PlatformKey, steamStaticAvatarID(u.SteamID64))
		displayURL, fallbackStatic := resolveSteamAvatarDisplay(staticURL, primaryURL)
		isManualAvatar := profileimage.HasManualProfileMarker(PlatformKey, u.SteamID64)
		miniHTMLForName := ReadCachedMiniprofileHTML(u.SteamID64)
		var avatarPending bool
		if ctx.effectiveCollect {
			avatarPending = steamAvatarPending(u.SteamID64, miniHTMLForName, ctx.st.SteamShowMiniProfile, ctx.st.SteamImageExpiryTime, isManualAvatar)
		}
		_, vacCached := ctx.vacKnown[u.SteamID64]
		metaPending := ctx.effectiveCollect && !vacCached

		note := ""
		if ctx.st.AccountNotes != nil {
			note = ctx.st.AccountNotes[u.SteamID64]
		}

		miniHTML := ApplySteamManualAvatarMiniprofile(miniHTMLForName, u.SteamID64)
		frameURL := ""
		if fu, ok := profileimage.FindCached(PlatformKey, u.SteamID64+"_frame"); ok {
			frameURL = fu
		}

		displayName := CachedCommunityDisplayName(u.SteamID64)
		if displayName == "" {
			displayName = displayPersona(u)
		}

		out = append(out, SteamAccountEnrichmentDTO{
			SteamID64:          u.SteamID64,
			DisplayName:        displayName,
			LastLogin:          formatLastLogin(u.Timestamp),
			Offline:            strings.TrimSpace(u.WantsOffline) == "1",
			ImageURL:           displayURL,
			StaticImageURL:     fallbackStatic,
			AvatarPending:      avatarPending,
			MetaPending:        metaPending,
			Vac:                v.Vac,
			Ltd:                v.Ltd,
			ShowSteamID:        ctx.st.SteamShowSteamID,
			ShowVAC:            ctx.st.SteamShowVAC,
			ShowLimited:        ctx.st.SteamShowLimited,
			ShowLastLogin:      ctx.st.SteamShowLastLogin,
			ShowAccUsername:    ctx.st.SteamShowAccUsername,
			CollectInfo:        ctx.st.CollectInfo,
			ShowShortNotes:     ctx.st.ShowShortNotes,
			Note:               note,
			AvatarFrameURL:     frameURL,
			MiniProfileHTML:    miniHTML,
			ShowMiniProfile:    ctx.st.SteamShowMiniProfile,
			ShowAvatarFrame:    ctx.st.SteamShowAvatarFrame,
			Tags:               ctx.tagByUID[u.SteamID64],
			ManualProfileImage: isManualAvatar,
		})
	}
	return out, nil
}

func mergeSteamAccountDTO(list SteamAccountListItemDTO, enrich SteamAccountEnrichmentDTO) AccountDTO {
	return AccountDTO{
		SteamID64:          list.SteamID64,
		PersonaName:        list.PersonaName,
		AccountName:        list.AccountName,
		DisplayName:        enrich.DisplayName,
		LastLogin:          enrich.LastLogin,
		Offline:            enrich.Offline,
		ImageURL:           enrich.ImageURL,
		StaticImageURL:     enrich.StaticImageURL,
		AvatarPending:      enrich.AvatarPending,
		MetaPending:        enrich.MetaPending,
		Vac:                enrich.Vac,
		Ltd:                enrich.Ltd,
		ShowSteamID:        enrich.ShowSteamID,
		ShowVAC:            enrich.ShowVAC,
		ShowLimited:        enrich.ShowLimited,
		ShowLastLogin:      enrich.ShowLastLogin,
		ShowAccUsername:    enrich.ShowAccUsername,
		CollectInfo:        enrich.CollectInfo,
		ShowShortNotes:     enrich.ShowShortNotes,
		Note:               enrich.Note,
		AvatarFrameURL:     enrich.AvatarFrameURL,
		MiniProfileHTML:    enrich.MiniProfileHTML,
		ShowMiniProfile:    enrich.ShowMiniProfile,
		ShowAvatarFrame:    enrich.ShowAvatarFrame,
		SyncError:          enrich.SyncError,
		CurrentSession:     list.CurrentSession,
		Tags:               enrich.Tags,
		ManualProfileImage: enrich.ManualProfileImage,
	}
}

func syncSteamPlatformCounts(accountCount int) {
	psPlat, perr := platform.LoadPlatformSettings(PlatformKey)
	sc, hot := 0, 0
	if perr == nil {
		sc, hot = accountlist.ShortcutCounts(psPlat.Shortcuts)
	}
	_ = stats.SyncPlatformCounts(PlatformKey, accountCount, sc, hot)
}
