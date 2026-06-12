package steam

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/basic"
	"TcNo-Acc-Switcher/internal/crashlog"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/profileimage"
	"TcNo-Acc-Switcher/internal/stats"
	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sync/semaphore"
)

const AccountUpdatedEvent = "steam-account-updated"

type AccountDTO struct {
	SteamID64 string `json:"steamId64"`

	PersonaName   string `json:"personaName"`
	AccountName   string `json:"accountName"`
	DisplayName   string `json:"displayName"`
	LastLogin     string `json:"lastLogin"`
	Offline       bool   `json:"offline"`
	ImageURL       string `json:"imageUrl"`
	StaticImageURL string `json:"staticImageUrl"`
	AvatarPending  bool   `json:"avatarPending"`
	MetaPending   bool   `json:"metaPending"`

	Vac bool `json:"vac"`
	Ltd bool `json:"ltd"`

	ShowSteamID     bool   `json:"showSteamId"`
	ShowVAC         bool   `json:"showVac"`
	ShowLimited     bool   `json:"showLimited"`
	ShowLastLogin   bool   `json:"showLastLogin"`
	ShowAccUsername bool   `json:"showAccUsername"`
	CollectInfo     bool   `json:"collectInfo"`
	ShowShortNotes  bool   `json:"showShortNotes"`
	Note            string `json:"note"`

	AvatarFrameURL  string `json:"avatarFrameUrl"`
	MiniProfileHTML string `json:"miniProfileHtml"`
	ShowMiniProfile bool   `json:"showMiniProfile"`
	ShowAvatarFrame bool   `json:"showAvatarFrame"`

	SyncError string `json:"syncError"`

	// CurrentSession: exactly one loginusers row has MostRecent=="1" and it is this account.
	CurrentSession bool `json:"currentSession"`

	Tags []basic.AccountTagDTO `json:"tags"`

	// ManualProfileImage: user-set avatar not replaced by refresh until removed.
	ManualProfileImage bool `json:"manualProfileImage"`
}

type AccountPatch struct {
	SteamID64 string `json:"steamId64"`

	ImageURL       string `json:"imageUrl"`
	StaticImageURL string `json:"staticImageUrl,omitempty"`
	Vac            bool   `json:"vac"`
	Ltd      bool   `json:"ltd"`

	AvatarPending bool `json:"avatarPending"`
	MetaPending   bool `json:"metaPending"`

	ManualProfileImage bool `json:"manualProfileImage,omitempty"`

	DisplayName string `json:"displayName,omitempty"`

	AvatarFrameURL  string `json:"avatarFrameUrl"`
	MiniProfileHTML string `json:"miniProfileHtml"`
	ShowMiniProfile bool   `json:"showMiniProfile"`
	ShowAvatarFrame bool   `json:"showAvatarFrame"`

	Error string `json:"error"`
}

type SteamService struct {
	mu sync.Mutex

	refreshMu      sync.Mutex
	refreshRunning bool
	refreshQueued  bool
	refreshTimer   *time.Timer
}

func NewSteamService() *SteamService {
	return &SteamService{}
}

func formatLastLogin(ts string) string {
	ts = strings.TrimSpace(ts)
	if ts == "" || ts == "0" {
		return ""
	}
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || sec <= 0 {
		return ""
	}
	return time.Unix(sec, 0).Local().Format(time.RFC3339)
}

func displayPersona(u LoginUser) string {
	n := strings.TrimSpace(u.PersonaName)
	if n != "" {
		return n
	}
	return strings.TrimSpace(u.AccountName)
}

const steamStaticAvatarSuffix = "_static"

func steamStaticAvatarID(steamID64 string) string {
	return strings.TrimSpace(steamID64) + steamStaticAvatarSuffix
}

func isAnimatedProfilePublicURL(publicURL string) bool {
	lu := strings.ToLower(strings.TrimSpace(publicURL))
	return strings.HasSuffix(lu, ".webm") || strings.HasSuffix(lu, ".mp4")
}

func resolveSteamAvatarDisplay(staticURL, primaryURL string) (imageURL, fallbackStatic string) {
	primaryURL = strings.TrimSpace(primaryURL)
	staticURL = strings.TrimSpace(staticURL)
	if primaryURL != "" {
		imageURL = primaryURL
	} else {
		imageURL = staticURL
	}
	fallbackStatic = staticURL
	if fallbackStatic == "" && imageURL != "" && !isAnimatedProfilePublicURL(imageURL) {
		fallbackStatic = imageURL
	}
	return imageURL, fallbackStatic
}

func steamAvatarPending(steamID64, miniProfileHTML string, useMiniProfile bool, maxAgeDays int, isManual bool) bool {
	if isManual {
		if p, ok := profileimage.CachedFilePath(PlatformKey, steamID64); ok {
			return profileimage.FileOlderThanDays(p, maxAgeDays)
		}
		return true
	}
	if useMiniProfile {
		staticPath, hasStatic := profileimage.CachedFilePath(PlatformKey, steamStaticAvatarID(steamID64))
		if !hasStatic || profileimage.FileOlderThanDays(staticPath, maxAgeDays) {
			return true
		}
		mediaSrc := ExtractMiniprofileAvatarMediaURL(miniProfileHTML)
		if mediaSrc == "" {
			return false
		}
		primaryPath, hasPrimary := profileimage.CachedFilePath(PlatformKey, steamID64)
		if !hasPrimary {
			return true
		}
		return profileimage.FileOlderThanDays(primaryPath, maxAgeDays)
	}
	if p, ok := profileimage.CachedFilePath(PlatformKey, steamID64); ok {
		return profileimage.FileOlderThanDays(p, maxAgeDays)
	}
	return true
}

func downloadSteamAccountAvatars(
	ctx context.Context,
	client *http.Client,
	steamID64, avatarFullURL, miniProfileHTML string,
	useMiniProfile bool,
	maxAgeDays int,
) (imageURL, staticURL string, err error) {
	avatarFullURL = strings.TrimSpace(avatarFullURL)
	steamID64 = strings.TrimSpace(steamID64)
	if avatarFullURL == "" {
		return "", "", fmt.Errorf("empty avatar URL")
	}
	if profileimage.HasManualProfileMarker(PlatformKey, steamID64) {
		if u, ok := profileimage.FindCached(PlatformKey, steamID64); ok {
			return u, u, nil
		}
		return "", "", fmt.Errorf("manual profile marker without cached file")
	}

	if !useMiniProfile {
		res, derr := profileimage.DownloadIfNeeded(ctx, client, PlatformKey, steamID64, avatarFullURL, maxAgeDays)
		if derr != nil {
			return "", "", derr
		}
		if res == nil {
			return "", "", fmt.Errorf("avatar download failed")
		}
		return res.PublicURL, "", nil
	}

	staticID := steamStaticAvatarID(steamID64)
	staticRes, derr := profileimage.DownloadIfNeeded(ctx, client, PlatformKey, staticID, avatarFullURL, maxAgeDays)
	if derr != nil {
		return "", "", derr
	}
	if staticRes == nil {
		return "", "", fmt.Errorf("static avatar download failed")
	}
	staticURL = staticRes.PublicURL

	mediaSrc := ExtractMiniprofileAvatarMediaURL(miniProfileHTML)
	if mediaSrc != "" {
		_ = profileimage.DeleteCachedImageFilesOnly(PlatformKey, steamID64)
		animRes, aerr := profileimage.DownloadIfNeeded(ctx, client, PlatformKey, steamID64, mediaSrc, maxAgeDays)
		if aerr == nil && animRes != nil {
			return animRes.PublicURL, staticURL, nil
		}
		if aerr != nil {
			steamLog.Debug("animated avatar download failed, using static fallback",
				slog.String("steamId", tailSteamID(steamID64)),
				slog.Any("err", aerr))
		}
	}

	return staticURL, staticURL, nil
}

func (s *SteamService) migrateExePathFromAppSettings(exeDir string, st *Settings, app *platform.AppSettings) error {
	exe := strings.TrimSpace(app.PlatformExePaths[platformName])
	if exe == "" {
		return nil
	}
	if strings.TrimSpace(st.FolderPath) != "" {
		return nil
	}
	st.FolderPath = NormalizeFolderPath(filepath.Dir(exe))
	delete(app.PlatformExePaths, platformName)
	if err := SaveSettings(*st); err != nil {
		return err
	}
	return platform.SaveAppSettings(exeDir, *app)
}

func (s *SteamService) GetSteamAccounts() ([]AccountDTO, error) {
	list, err := s.GetSteamAccountsList()
	if err != nil {
		return nil, err
	}
	enrich, err := s.GetSteamAccountsEnrichment()
	if err != nil {
		return nil, err
	}
	enrichByID := make(map[string]SteamAccountEnrichmentDTO, len(enrich))
	for _, row := range enrich {
		enrichByID[row.SteamID64] = row
	}
	out := make([]AccountDTO, 0, len(list))
	for _, row := range list {
		out = append(out, mergeSteamAccountDTO(row, enrichByID[row.SteamID64]))
	}
	syncSteamPlatformCounts(len(out))
	return out, nil
}

func (s *SteamService) SaveSteamAccountOrder(ids []string) error {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return err
	}
	app, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		return err
	}
	st, err := LoadSettings()
	if err != nil {
		return err
	}
	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		return err
	}
	root, err := ResolveInstallFolder(exeDir, st, app, raw)
	if err != nil {
		return err
	}
	users, err := ParseLoginUsers(LoginUsersPath(root))
	if err != nil {
		return err
	}
	valid := make(map[string]struct{}, len(users))
	for _, u := range users {
		valid[u.SteamID64] = struct{}{}
	}
	if len(ids) != len(valid) {
		return errors.New("order length does not match accounts")
	}
	seen := make(map[string]struct{})
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if _, ok := valid[id]; !ok {
			return fmt.Errorf("unknown steam id in order: %s", id)
		}
		if _, dup := seen[id]; dup {
			return errors.New("duplicate steam id in order")
		}
		seen[id] = struct{}{}
	}
	return SaveOrder(ids)
}

func (s *SteamService) GetSteamSettings() (Settings, error) {
	return LoadSettings()
}

func (s *SteamService) SaveSteamSettings(st Settings) error {
	return SaveSettings(st)
}

func (s *SteamService) RefreshVACStatus() error {
	if err := ClearVACProfileCaches(); err != nil {
		return err
	}
	s.StartSteamProfileRefresh()
	return nil
}

func (s *SteamService) RefreshAllSteamImages() error {
	_ = ClearAllMiniprofileHTMLCache()
	dir, err := profileimage.ProfileDir(PlatformKey)
	if err != nil {
		return err
	}
	if _, err := os.ReadDir(dir); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			s.StartSteamProfileRefresh()
			return nil
		}
		return err
	}
	if err := profileimage.DeleteAutomatedProfileCaches(PlatformKey); err != nil {
		return err
	}
	s.StartSteamProfileRefresh()
	return nil
}

func clearExpiredSteamProfileAssets(steamID64 string, maxAgeDays int) {
	ids := []string{
		steamID64,
		steamStaticAvatarID(steamID64),
		steamID64 + "_frame",
		steamID64 + "_nameplate",
		steamID64 + "_featuredbadge",
	}
	removed := deleteMiniprofileCacheIfOlder(steamID64, maxAgeDays)
	for _, id := range ids {
		if id == steamID64 && profileimage.HasManualProfileMarker(PlatformKey, steamID64) {
			continue
		}
		if p, ok := profileimage.CachedFilePath(PlatformKey, id); ok && profileimage.FileOlderThanDays(p, maxAgeDays) {
			_ = profileimage.DeleteCached(PlatformKey, id)
			removed = true
		}
	}
	if removed {
		deleteMiniprofileCache(steamID64)
	}
}

func (s *SteamService) StartSteamProfileRefresh() {
	s.refreshMu.Lock()
	if s.refreshTimer != nil {
		s.refreshTimer.Stop()
	}
	s.refreshTimer = time.AfterFunc(500*time.Millisecond, func() {
		defer crashlog.Capture()
		s.runProfileRefresh()
	})
	s.refreshMu.Unlock()
}

func (s *SteamService) runProfileRefresh() {
	s.refreshMu.Lock()
	if s.refreshRunning {
		s.refreshQueued = true
		s.refreshMu.Unlock()
		steamLog.Debug("profile refresh coalesced: already running")
		return
	}
	s.refreshRunning = true
	s.refreshMu.Unlock()

	defer func() {
		var again bool
		s.refreshMu.Lock()
		s.refreshRunning = false
		again = s.refreshQueued
		s.refreshQueued = false
		s.refreshMu.Unlock()
		if again {
			s.StartSteamProfileRefresh()
		}
	}()

	steamLog.Info("background profile refresh started")

	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		steamLog.Error("ResolveExeDir failed", slog.Any("err", err))
		return
	}
	app, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		steamLog.Error("LoadAppSettings failed", slog.Any("err", err))
		return
	}
	st, err := LoadSettings()
	if err != nil {
		steamLog.Error("LoadSettings (Steam) failed", slog.Any("err", err))
		return
	}
	if !st.CollectInfo {
		steamLog.Info("profile refresh skipped: CollectInfo is false")
		return
	}
	if app.OfflineMode {
		steamLog.Info("profile refresh skipped: offline mode")
		return
	}

	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		steamLog.Error("load platforms config failed", slog.Any("err", err))
		return
	}
	root, err := ResolveInstallFolder(exeDir, st, app, raw)
	if err != nil {
		steamLog.Error("ResolveInstallFolder failed", slog.Any("err", err))
		return
	}
	if root == "" {
		steamLog.Error("steam root empty after ResolveInstallFolder")
		return
	}
	users, err := ParseLoginUsers(LoginUsersPath(root))
	if err != nil {
		steamLog.Error("ParseLoginUsers failed in refresh", slog.Any("err", err))
		return
	}
	if len(users) == 0 {
		steamLog.Warn("no Steam users to refresh (loginusers empty)")
		return
	}

	steamLog.Info("refreshing Steam profiles", slog.Int("accounts", len(users)), slog.Int("concurrency", 5))

	vacRows, _ := LoadVacCache(st.SteamImageExpiryTime)
	vm := vacMap(vacRows)
	var vmMu sync.Mutex

	ctx := context.Background()
	sem := semaphore.NewWeighted(5)
	var wg sync.WaitGroup

	for _, u := range users {
		u := u
		wg.Add(1)
		go func() {
			defer crashlog.Capture()
			defer wg.Done()
			_ = sem.Acquire(ctx, 1)
			defer sem.Release(1)

			clearExpiredSteamProfileAssets(u.SteamID64, st.SteamImageExpiryTime)

			vmMu.Lock()
			prev := vm[u.SteamID64]
			vmMu.Unlock()

			patch := AccountPatch{
				SteamID64:       u.SteamID64,
				Vac:             prev.Vac,
				Ltd:             prev.Ltd,
				ShowMiniProfile: st.SteamShowMiniProfile,
				ShowAvatarFrame: st.SteamShowAvatarFrame,
			}

			xctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			fields, err := FetchProfileXML(xctx, appclient.Shared, u.SteamID64)
			cancel()

			if err != nil {
				steamLog.Warn("community profile XML failed",
					slog.String("steamId", tailSteamID(u.SteamID64)),
					slog.Any("err", err))
				patch.Error = err.Error()
				patch.MetaPending = false
				patch.AvatarPending = false
				s.emit(patch)
				return
			}
			if fields.Private {
				steamLog.Info("community profile private or blocked",
					slog.String("steamId", tailSteamID(u.SteamID64)))
				patch.Error = "Profile is private or unavailable"
				patch.MetaPending = false
				patch.AvatarPending = false
				s.emit(patch)
				return
			}

			patch.Vac = fields.VacBanned
			patch.Ltd = fields.Limited
			patch.MetaPending = false
			patch.Error = ""
			patch.DisplayName = fields.CommunityDisplayName
			patch.ShowMiniProfile = st.SteamShowMiniProfile
			patch.ShowAvatarFrame = st.SteamShowAvatarFrame

			vmMu.Lock()
			vm[u.SteamID64] = VacEntry{SteamID: u.SteamID64, Vac: patch.Vac, Ltd: patch.Ltd}
			vmMu.Unlock()

			if st.SteamShowMiniProfile || st.SteamShowAvatarFrame {
				mctx, mcancel := context.WithTimeout(ctx, 15*time.Second)
				frameSrc, miniHTML, mErr := FetchMiniprofile(mctx, appclient.Shared, u.SteamID64, st.SteamImageExpiryTime)
				mcancel()
				if mErr != nil {
					steamLog.Warn("miniprofile fetch failed",
						slog.String("steamId", tailSteamID(u.SteamID64)),
						slog.Any("err", mErr))
				} else {
					patch.MiniProfileHTML = miniHTML
					if n := ExtractMiniprofileDisplayName(miniHTML); n != "" {
						patch.DisplayName = n
					}
					if st.SteamShowAvatarFrame && strings.TrimSpace(frameSrc) != "" {
						fctx, fcancel := context.WithTimeout(ctx, 15*time.Second)
						res, derr := profileimage.DownloadIfNeeded(fctx, appclient.Shared, PlatformKey, u.SteamID64+"_frame", frameSrc, st.SteamImageExpiryTime)
						fcancel()
						if derr == nil && res != nil {
							patch.AvatarFrameURL = res.PublicURL
						} else if derr != nil {
							steamLog.Debug("avatar frame download failed",
								slog.String("steamId", tailSteamID(u.SteamID64)),
								slog.Any("err", derr))
						}
					} else if st.SteamShowAvatarFrame {
						if fu, ok := profileimage.FindCached(PlatformKey, u.SteamID64+"_frame"); ok {
							patch.AvatarFrameURL = fu
						}
					}
				}
			}

			if strings.TrimSpace(fields.AvatarFullURL) == "" {
				steamLog.Warn("no avatar URL in profile XML",
					slog.String("steamId", tailSteamID(u.SteamID64)))
				patch.Error = "No avatar URL in profile"
				patch.AvatarPending = false
				s.emit(patch)
				return
			}

			useMiniProfile := st.SteamShowMiniProfile

			if profileimage.HasManualProfileMarker(PlatformKey, u.SteamID64) {
				if cachedURL, hit := profileimage.FindCached(PlatformKey, u.SteamID64); hit {
					patch.ImageURL = cachedURL
					patch.StaticImageURL = cachedURL
					patch.AvatarPending = false
					patch.Error = ""
					s.emit(patch)
					return
				}
			}

			if !steamAvatarPending(u.SteamID64, patch.MiniProfileHTML, useMiniProfile, st.SteamImageExpiryTime, false) {
				primaryURL, _ := profileimage.FindCached(PlatformKey, u.SteamID64)
				staticURL, _ := profileimage.FindCached(PlatformKey, steamStaticAvatarID(u.SteamID64))
				patch.ImageURL, patch.StaticImageURL = resolveSteamAvatarDisplay(staticURL, primaryURL)
				patch.AvatarPending = false
				patch.Error = ""
				s.emit(patch)
				return
			}

			patch.AvatarPending = true
			s.emit(patch)

			ictx, icancel := context.WithTimeout(ctx, 20*time.Second)
			imageURL, staticURL, err := downloadSteamAccountAvatars(
				ictx, appclient.Shared, u.SteamID64, fields.AvatarFullURL, patch.MiniProfileHTML, useMiniProfile, st.SteamImageExpiryTime,
			)
			icancel()
			if err == nil {
				patch.ImageURL = imageURL
				patch.StaticImageURL = staticURL
				patch.AvatarPending = false
				patch.Error = ""
				steamLog.Info("avatar cached",
					slog.String("steamId", tailSteamID(u.SteamID64)),
					slog.String("url", imageURL))
			} else {
				steamLog.Warn("avatar download failed",
					slog.String("steamId", tailSteamID(u.SteamID64)),
					slog.Any("err", err))
				patch.Error = err.Error()
				patch.AvatarPending = false
			}
			s.emit(patch)
		}()
	}
	wg.Wait()

	rows := make([]VacEntry, 0, len(users))
	for _, u := range users {
		if e, ok := vm[u.SteamID64]; ok {
			rows = append(rows, e)
		}
	}
	if err := SaveVacCache(rows); err != nil {
		steamLog.Error("SaveVacCache failed", slog.Any("err", err))
	} else {
		steamLog.Info("profile refresh finished", slog.Int("vacRows", len(rows)))
	}
}

func (s *SteamService) emit(p AccountPatch) {
	app := application.Get()
	if app == nil {
		steamLog.Warn("emit steam-account-updated skipped: application not ready",
			slog.String("steamId", tailSteamID(p.SteamID64)))
		return
	}
	id := strings.TrimSpace(p.SteamID64)
	if id != "" {
		p.ManualProfileImage = profileimage.HasManualProfileMarker(PlatformKey, id)
		p.MiniProfileHTML = ApplySteamManualAvatarMiniprofile(p.MiniProfileHTML, id)
	}
	app.Event.Emit(AccountUpdatedEvent, p)
}

func (s *SteamService) GetSteamIDFormats(id64 string) (SteamIDFormats, error) {
	return FormatsFromID64(strings.TrimSpace(id64))
}

func (s *SteamService) SwapToSteamAccount(steamID64 string, personaState int, extraLaunchArgs []string) error {
	return SwapToAccount(strings.TrimSpace(steamID64), personaState, extraLaunchArgs)
}

func (s *SteamService) SteamAddNew() error {
	return SwapToAccount("", -1, nil)
}

func (s *SteamService) LaunchSteam() error {
	return LaunchSteamOnly(nil)
}

func (s *SteamService) ForgetSteamAccount(steamID64 string) error {
	steamID64 = strings.TrimSpace(steamID64)
	if steamID64 == "" {
		return errors.New("empty steam id")
	}
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return err
	}
	app, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		return err
	}
	st, err := LoadSettings()
	if err != nil {
		return err
	}
	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		return err
	}
	root, err := ResolveInstallFolder(exeDir, st, app, raw)
	if err != nil {
		return err
	}
	if root == "" {
		return fmt.Errorf("steam install folder not found")
	}
	if err := RemoveSteamAccountFromVDF(root, steamID64); err != nil {
		return err
	}
	_ = profileimage.DeleteCached(PlatformKey, steamID64)
	_ = profileimage.DeleteCached(PlatformKey, steamStaticAvatarID(steamID64))
	_ = profileimage.DeleteCached(PlatformKey, steamID64+"_frame")
	_ = profileimage.DeleteCached(PlatformKey, steamID64+"_nameplate")
	_ = profileimage.DeleteCached(PlatformKey, steamID64+"_featuredbadge")
	deleteMiniprofileCache(steamID64)
	_ = basic.ForgetAccountTagAssignments(PlatformKey, steamID64)
	s.StartSteamProfileRefresh()
	return nil
}

func (s *SteamService) steamInstallRoot() (string, error) {
	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		return "", err
	}
	app, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		return "", err
	}
	st, err := LoadSettings()
	if err != nil {
		return "", err
	}
	raw, err := platform.LoadPlatformsJSON(exeDir)
	if err != nil {
		return "", err
	}
	return ResolveInstallFolder(exeDir, st, app, raw)
}

func (s *SteamService) GetInstalledGames() ([]InstalledGameInfo, error) {
	root, err := s.steamInstallRoot()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("steam install folder not found")
	}
	return BuildInstalledGamesList(context.Background(), root)
}

func (s *SteamService) OpenUserdataFolder(steamID64 string) error {
	f, err := FormatsFromID64(strings.TrimSpace(steamID64))
	if err != nil {
		return err
	}
	root, err := s.steamInstallRoot()
	if err != nil {
		return err
	}
	if strings.TrimSpace(root) == "" {
		return fmt.Errorf("steam install folder not found")
	}
	ud := filepath.Join(root, "userdata", f.ID32)
	return platform.OpenPathInFileManager(ud)
}

func (s *SteamService) LoginAndLaunchGame(steamID64 string, personaState int, appID string) error {
	steamID64 = strings.TrimSpace(steamID64)
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return errors.New("empty app id")
	}
	// Match shortcut behavior: if loginusers.vdf already marks this account as the active session,
	// only open the game URL — do not run SwapToAccount (which kills and restarts Steam).
	active, errActive := CurrentLiveSteamID64()
	skipSwap := errActive == nil && active != "" && strings.EqualFold(strings.TrimSpace(active), steamID64)
	if !skipSwap {
		if err := SwapToAccount(steamID64, personaState, nil); err != nil {
			return err
		}
	}
	url := "steam://rungameid/" + appID
	if err := winutil.Start("cmd.exe", []string{"/c", "start", "", url}, winutil.StartOpts{}); err != nil {
		return err
	}
	_ = stats.IncrementGamesLaunched(PlatformKey)
	return nil
}

func (s *SteamService) ChangeAccountImage(steamID64, sourcePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	steamID64 = strings.TrimSpace(steamID64)
	sourcePath = strings.TrimSpace(sourcePath)
	if steamID64 == "" || sourcePath == "" {
		return errors.New("invalid change image parameters")
	}
	if err := profileimage.WriteManualProfileMarker(PlatformKey, steamID64); err != nil {
		return err
	}
	if err := profileimage.CacheLocalFileForUser(PlatformKey, steamID64, sourcePath); err != nil {
		_ = profileimage.ClearManualProfileMarker(PlatformKey, steamID64)
		return err
	}
	return nil
}

// ClearManualAccountProfileImage removes a user-set Steam avatar so automated images apply again.
func (s *SteamService) ClearManualAccountProfileImage(steamID64 string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	steamID64 = strings.TrimSpace(steamID64)
	if steamID64 == "" {
		return errors.New("empty steam id")
	}
	if err := profileimage.DeleteCached(PlatformKey, steamID64); err != nil {
		return err
	}
	s.StartSteamProfileRefresh()
	return nil
}
