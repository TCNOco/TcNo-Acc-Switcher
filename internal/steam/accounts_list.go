package steam

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/accountlist"
	"TcNo-Acc-Switcher/internal/basic"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/profileimage"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/stats"
	"TcNo-Acc-Switcher/internal/steamguard/cs2cooldown"
	steamguardregistry "TcNo-Acc-Switcher/internal/steamguard/registry"
)

// SteamAccountListItemDTO is the fast Steam account list payload.
//
// No DisplayName: that is the community name, and only the enrichment pass
// knows it. This payload can only see the loginusers.vdf persona, so sending it
// as a display name would let a list refresh overwrite the real one on screen.
type SteamAccountListItemDTO struct {
	SteamID64         string `json:"steamId64"`
	PersonaName       string `json:"personaName"`
	AccountName       string `json:"accountName"`
	CurrentSession    bool   `json:"currentSession"`
	HasSteamGuard     bool   `json:"hasSteamGuard"`
	SteamGuardPending bool   `json:"steamGuardPending"`
	// SteamGuardLoginOnly is a vault record with a session but no authenticator.
	// The toolbar entry keys off it so an account list containing only
	// login-only records can still reach the Steam Guard window.
	SteamGuardLoginOnly bool `json:"steamGuardLoginOnly"`
}

// SteamAccountEnrichmentDTO carries slower per-account Steam metadata.
type SteamAccountEnrichmentDTO struct {
	SteamID64 string `json:"steamId64"`

	DisplayName    string `json:"displayName"`
	LastLogin      string `json:"lastLogin"`
	Offline        bool   `json:"offline"`
	ImageURL       string `json:"imageUrl"`
	StaticImageURL string `json:"staticImageUrl"`
	AvatarPending  bool   `json:"avatarPending"`
	MetaPending    bool   `json:"metaPending"`
	Vac            bool   `json:"vac"`
	Ltd            bool   `json:"ltd"`
	// HasVisibleBan reports a ban the global settings are set to show, whether
	// or not this account is hidden. It is what decides that the context menu
	// has anything to offer; BanStatusHidden decides which way it reads.
	HasVisibleBan      bool                  `json:"hasVisibleBan"`
	BanStatusHidden    bool                  `json:"banStatusHidden"`
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
	ShowSteamGuardLock bool                  `json:"showSteamGuardLock"`
	SyncError          string                `json:"syncError"`
	Tags               []basic.AccountTagDTO `json:"tags"`
	ManualProfileImage bool                  `json:"manualProfileImage"`

	// CS2CooldownExpiresAt is RFC3339 UTC, empty when there is no cooldown. An
	// absolute instant rather than a remaining duration, so the tile can count
	// it down live without asking again.
	CS2CooldownExpiresAt string `json:"cs2CooldownExpiresAt"`
	CS2CooldownPermanent bool   `json:"cs2CooldownPermanent"`
	ShowCS2Cooldown      bool   `json:"showCs2Cooldown"`
}

type steamListContext struct {
	users            []LoginUser
	activeSteamID    string
	st               Settings
	app              platform.AppSettings
	effectiveCollect bool
	vacMap           map[string]VacEntry
	vacKnown         map[string]struct{}
	hiddenBans       map[string]struct{}
	tagByUID         map[string][]basic.AccountTagDTO
	cooldowns        map[string]cs2cooldown.Entry
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

	hiddenBans := make(map[string]struct{}, len(st.HiddenBanStatus))
	for _, id := range st.HiddenBanStatus {
		if id = strings.TrimSpace(id); id != "" {
			hiddenBans[id] = struct{}{}
		}
	}

	tagByUID, _ := basic.BuildAccountTagMap(PlatformKey)

	// A corrupt cooldown cache must not fail the account list; an empty map just
	// means no cooldown lines until the next sweep rewrites the file.
	cooldowns, err := cs2cooldown.Load()
	if err != nil {
		steamLog.Warn("CS2 cooldown cache unavailable", slog.Any("err", err))
		cooldowns = map[string]cs2cooldown.Entry{}
	}

	return &steamListContext{
		users:            users,
		activeSteamID:    activeSteamID,
		st:               st,
		app:              app,
		effectiveCollect: st.CollectInfo && !app.OfflineMode,
		vacMap:           vm,
		vacKnown:         vacKnown,
		hiddenBans:       hiddenBans,
		tagByUID:         tagByUID,
		cooldowns:        cooldowns,
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

	states := loadSteamGuardAccountStates()
	out := buildSteamAccountListItems(ctx.users, ctx.activeSteamID, states)
	if len(out) > 0 {
		syncSteamPlatformCounts(len(out))
	}
	return out, nil
}

func loadSteamGuardAccountStates() map[string]SteamGuardAccountState {
	entries, err := steamguardregistry.Load()
	if err != nil {
		steamLog.Warn("Steam Guard account state unavailable", slog.Any("err", err))
		return nil
	}
	states := make(map[string]SteamGuardAccountState, len(entries))
	for _, entry := range entries {
		states[entry.SteamID64] = SteamGuardAccountState{
			HasSteamGuard: entry.State == steamguardregistry.StateActive,
			Pending:       entry.State == steamguardregistry.StatePending,
			LoginOnly:     entry.State == steamguardregistry.StateLoginOnly,
		}
	}
	return states
}

func buildSteamAccountListItems(users []LoginUser, activeSteamID string, states map[string]SteamGuardAccountState) []SteamAccountListItemDTO {
	out := make([]SteamAccountListItemDTO, 0, len(users))
	for _, u := range users {
		persona := displayPersona(u)
		guardState := states[u.SteamID64]
		out = append(out, SteamAccountListItemDTO{
			SteamID64:           u.SteamID64,
			PersonaName:         persona,
			AccountName:         strings.TrimSpace(u.AccountName),
			CurrentSession:      activeSteamID != "" && u.SteamID64 == activeSteamID,
			HasSteamGuard:       guardState.HasSteamGuard,
			SteamGuardPending:   guardState.Pending,
			SteamGuardLoginOnly: guardState.LoginOnly,
		})
	}
	return out
}

func (s *SteamService) GetSteamAccountsEnrichment() ([]SteamAccountEnrichmentDTO, error) {
	if err := security.RequireUnlocked(); err != nil {
		return nil, err
	}
	ctx, err := s.buildSteamListContext()
	if err != nil {
		return nil, err
	}

	// Every avatar variant this loop asks about (primary, static, frame) lives
	// in the same platform directory, so one read answers all of them for all
	// accounts. Probing per account costs upwards of thirty stat calls each.
	avatars, err := profileimage.NewSnapshot(PlatformKey)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	out := make([]SteamAccountEnrichmentDTO, 0, len(ctx.users))
	for _, u := range ctx.users {
		v := ctx.vacMap[u.SteamID64]
		primaryURL, _ := avatars.FindCached(u.SteamID64)
		staticURL, _ := avatars.FindCached(steamStaticAvatarID(u.SteamID64))
		displayURL, fallbackStatic := resolveSteamAvatarDisplay(staticURL, primaryURL)
		isManualAvatar := avatars.HasManualProfileMarker(u.SteamID64)
		miniHTMLForName := ReadCachedMiniprofileHTML(u.SteamID64)
		var avatarPending bool
		if ctx.effectiveCollect {
			avatarPending = steamAvatarPending(avatars, u.SteamID64, miniHTMLForName, ctx.st.SteamShowMiniProfile, ctx.st.SteamImageExpiryTime, isManualAvatar)
		}
		_, vacCached := ctx.vacKnown[u.SteamID64]
		metaPending := ctx.effectiveCollect && !vacCached

		note := ""
		if ctx.st.AccountNotes != nil {
			note = ctx.st.AccountNotes[u.SteamID64]
		}
		_, banHidden := ctx.hiddenBans[u.SteamID64]
		bans := banDisplayFor(v, ctx.st.SteamShowVAC, ctx.st.SteamShowLimited, banHidden)

		miniHTML := ApplySteamManualAvatarMiniprofile(miniHTMLForName, u.SteamID64)
		frameURL := ""
		if fu, ok := avatars.FindCached(u.SteamID64 + "_frame"); ok {
			frameURL = fu
		}

		// Reuses the miniprofile HTML read above; CachedCommunityDisplayName
		// would otherwise read and re-parse the same file a second time.
		displayName := communityDisplayNameFrom(miniHTMLForName, u.SteamID64)
		if displayName == "" {
			displayName = displayPersona(u)
		}

		// Only send a cooldown that is still running. An entry whose expiry has
		// passed is history, and the frontend would drop it anyway.
		cooldownExpiry, cooldownPermanent := "", false
		if entry, ok := ctx.cooldowns[u.SteamID64]; ok && entry.Active(now) {
			cooldownPermanent = entry.Permanent
			if !entry.Permanent {
				cooldownExpiry = time.Unix(entry.CooldownExpiresAt, 0).UTC().Format(time.RFC3339)
			}
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
			HasVisibleBan:      bans.HasVisibleBan,
			BanStatusHidden:    bans.Hidden,
			ShowSteamID:        ctx.st.SteamShowSteamID,
			ShowVAC:            bans.ShowVAC,
			ShowLimited:        bans.ShowLimited,
			ShowLastLogin:      ctx.st.SteamShowLastLogin,
			ShowAccUsername:    ctx.st.SteamShowAccUsername,
			CollectInfo:        ctx.st.CollectInfo,
			ShowShortNotes:     ctx.st.ShowShortNotes,
			Note:               note,
			AvatarFrameURL:     frameURL,
			MiniProfileHTML:    miniHTML,
			ShowMiniProfile:    ctx.st.SteamShowMiniProfile,
			ShowAvatarFrame:    ctx.st.SteamShowAvatarFrame,
			ShowSteamGuardLock: ctx.st.SteamShowSteamGuardLock,
			Tags:               ctx.tagByUID[u.SteamID64],
			ManualProfileImage: isManualAvatar,

			CS2CooldownExpiresAt: cooldownExpiry,
			CS2CooldownPermanent: cooldownPermanent,
			ShowCS2Cooldown:      ctx.st.SteamCollectCS2Cooldowns,
			// Both settings gate this: the rank is a by-product of the cooldown
			// sweep, so with collection off there is nothing keeping it current.
		})
	}
	return out, nil
}

// banDisplay is what the account list says about one account's bans.
type banDisplay struct {
	// ShowVAC and ShowLimited gate every place a ban is painted - the name
	// colour, the avatar, the Steam Guard summary, the re-render key. Hiding is
	// applied here rather than at each of them, so they cannot disagree.
	ShowVAC     bool
	ShowLimited bool
	// HasVisibleBan reports a ban the global settings are set to show, ignoring
	// whether this account is hidden. It is what decides that the context menu
	// has anything to offer, and stays true once hidden so the offer can be
	// reversed.
	HasVisibleBan bool
	Hidden        bool
}

func banDisplayFor(v VacEntry, showVAC, showLimited, hidden bool) banDisplay {
	return banDisplay{
		ShowVAC:       showVAC && !hidden,
		ShowLimited:   showLimited && !hidden,
		HasVisibleBan: (v.Vac && showVAC) || (v.Ltd && showLimited),
		Hidden:        hidden,
	}
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
		HasVisibleBan:      enrich.HasVisibleBan,
		BanStatusHidden:    enrich.BanStatusHidden,
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
		ShowSteamGuardLock: enrich.ShowSteamGuardLock,
		SyncError:          enrich.SyncError,
		CurrentSession:     list.CurrentSession,
		Tags:               enrich.Tags,
		ManualProfileImage: enrich.ManualProfileImage,

		CS2CooldownExpiresAt: enrich.CS2CooldownExpiresAt,
		CS2CooldownPermanent: enrich.CS2CooldownPermanent,
		ShowCS2Cooldown:      enrich.ShowCS2Cooldown,
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
