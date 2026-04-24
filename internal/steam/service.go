package steam

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/profileimage"
	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sync/semaphore"
)

// AccountUpdatedEvent is the Wails event name for per-row patches.
const AccountUpdatedEvent = "steam-account-updated"

// AccountDTO is the initial snapshot and list row model.
type AccountDTO struct {
	SteamID64 string `json:"steamId64"`

	PersonaName   string `json:"personaName"`
	AccountName   string `json:"accountName"`
	DisplayName   string `json:"displayName"`
	LastLogin     string `json:"lastLogin"`
	Offline       bool   `json:"offline"`
	ImageURL      string `json:"imageUrl"`
	AvatarPending bool   `json:"avatarPending"`
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

	// SyncError is set when background profile/avatar fetch fails (shown in UI; also logged).
	SyncError string `json:"syncError"`

	// CurrentSession is true when this row is the active Steam session per loginusers.vdf
	// True when loginusers.vdf has exactly one user with MostRecent=="1" (matches that row).
	CurrentSession bool `json:"currentSession"`
}

// AccountPatch is emitted when background work updates one account.
type AccountPatch struct {
	SteamID64 string `json:"steamId64"`

	ImageURL string `json:"imageUrl"`
	Vac      bool   `json:"vac"`
	Ltd      bool   `json:"ltd"`

	AvatarPending bool `json:"avatarPending"`
	MetaPending   bool `json:"metaPending"`

	DisplayName string `json:"displayName,omitempty"`

	Error string `json:"error"`
}

// SteamService exposes Steam accounts to the Wails frontend.
type SteamService struct {
	mu sync.Mutex

	refreshMu      sync.Mutex
	refreshRunning bool
	refreshQueued  bool // run again after the current refresh finishes (coalesce overlapping StartSteamProfileRefresh)
}

// NewSteamService constructs the service. Outbound HTTP uses [appclient.Shared].
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

// GetSteamAccounts returns the ordered account list for first paint.
func (s *SteamService) GetSteamAccounts() ([]AccountDTO, error) {
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

	pj, err := platform.ResolvePlatformsJSONPath(exeDir)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(pj)
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
	steamLog.Info("loading Steam accounts", slog.String("steamRoot", root), slog.String("loginusers", loginPath))

	users, err := ParseLoginUsers(loginPath)
	if err != nil {
		steamLog.Error("ParseLoginUsers failed", slog.String("path", loginPath), slog.Any("err", err))
		return nil, err
	}
	steamLog.Info("parsed loginusers.vdf", slog.Int("userCount", len(users)))

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

	out := make([]AccountDTO, 0, len(users))
	for _, u := range users {
		v := vm[u.SteamID64]
		imgURL, hasImg := profileimage.FindCached(PlatformKey, u.SteamID64)
		var avatarPending bool
		if st.CollectInfo {
			if !hasImg {
				avatarPending = true
			} else if p, ok := profileimage.CachedFilePath(PlatformKey, u.SteamID64); ok {
				avatarPending = profileimage.FileOlderThanDays(p, st.SteamImageExpiryTime)
			}
		}
		_, vacCached := vacKnown[u.SteamID64]
		metaPending := st.CollectInfo && !vacCached

		note := ""
		if st.AccountNotes != nil {
			note = st.AccountNotes[u.SteamID64]
		}

		dto := AccountDTO{
			SteamID64:       u.SteamID64,
			PersonaName:     displayPersona(u),
			AccountName:     strings.TrimSpace(u.AccountName),
			DisplayName:     CachedCommunityDisplayName(u.SteamID64),
			LastLogin:       formatLastLogin(u.Timestamp),
			Offline:         strings.TrimSpace(u.WantsOffline) == "1",
			ImageURL:        imgURL,
			AvatarPending:   avatarPending,
			MetaPending:     metaPending,
			Vac:             v.Vac,
			Ltd:             v.Ltd,
			ShowSteamID:     st.SteamShowSteamID,
			ShowVAC:         st.SteamShowVAC,
			ShowLimited:     st.SteamShowLimited,
			ShowLastLogin:   st.SteamShowLastLogin,
			ShowAccUsername: st.SteamShowAccUsername,
			CollectInfo:     st.CollectInfo,
			ShowShortNotes:  st.ShowShortNotes,
			Note:            note,
			CurrentSession:  activeSteamID != "" && u.SteamID64 == activeSteamID,
		}
		out = append(out, dto)
	}
	steamLog.Info("GetSteamAccounts done", slog.Int("rows", len(out)))
	return out, nil
}

// SaveSteamAccountOrder persists account order by SteamID64.
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
	pj, err := platform.ResolvePlatformsJSONPath(exeDir)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(pj)
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

// GetSteamSettings returns current Steam settings.
func (s *SteamService) GetSteamSettings() (Settings, error) {
	return LoadSettings()
}

// SaveSteamSettings writes Steam settings.
func (s *SteamService) SaveSteamSettings(st Settings) error {
	return SaveSettings(st)
}

// RefreshVACStatus clears VAC/profile XML caches and triggers a background profile refresh.
func (s *SteamService) RefreshVACStatus() error {
	if err := ClearVACProfileCaches(); err != nil {
		return err
	}
	s.StartSteamProfileRefresh()
	return nil
}

// RefreshAllSteamImages deletes cached avatar files for all Steam accounts and triggers a refresh.
func (s *SteamService) RefreshAllSteamImages() error {
	dir, err := profileimage.ProfileDir(PlatformKey)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			s.StartSteamProfileRefresh()
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
	s.StartSteamProfileRefresh()
	return nil
}

// StartSteamProfileRefresh fetches missing avatars and ban info in the background.
func (s *SteamService) StartSteamProfileRefresh() {
	go s.runProfileRefresh()
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
			go s.runProfileRefresh()
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

	pj, err := platform.ResolvePlatformsJSONPath(exeDir)
	if err != nil {
		steamLog.Error("ResolvePlatformsJSONPath failed", slog.Any("err", err))
		return
	}
	raw, err := os.ReadFile(pj)
	if err != nil {
		steamLog.Error("read Platforms.json failed", slog.String("path", pj), slog.Any("err", err))
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
			defer wg.Done()
			_ = sem.Acquire(ctx, 1)
			defer sem.Release(1)

			vmMu.Lock()
			prev := vm[u.SteamID64]
			vmMu.Unlock()

			patch := AccountPatch{SteamID64: u.SteamID64, Vac: prev.Vac, Ltd: prev.Ltd}

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

			vmMu.Lock()
			vm[u.SteamID64] = VacEntry{SteamID: u.SteamID64, Vac: patch.Vac, Ltd: patch.Ltd}
			vmMu.Unlock()

			if strings.TrimSpace(fields.AvatarFullURL) == "" {
				steamLog.Warn("no avatar URL in profile XML",
					slog.String("steamId", tailSteamID(u.SteamID64)))
				patch.Error = "No avatar URL in profile"
				patch.AvatarPending = false
				s.emit(patch)
				return
			}

			// If JPEG/PNG on disk is still within expiry, DownloadIfNeeded would return without
			// downloading — avoid emitting AvatarPending first, which forces placeholder + "Updating…"
			// and can appear stuck if the follow-up emit is missed.
			if p, ok := profileimage.CachedFilePath(PlatformKey, u.SteamID64); ok {
				if !profileimage.FileOlderThanDays(p, st.SteamImageExpiryTime) {
					if cachedURL, hit := profileimage.FindCached(PlatformKey, u.SteamID64); hit {
						patch.ImageURL = cachedURL
						patch.AvatarPending = false
						s.emit(patch)
						return
					}
				}
			}

			patch.AvatarPending = true
			s.emit(patch)

			ictx, icancel := context.WithTimeout(ctx, 15*time.Second)
			res, err := profileimage.DownloadIfNeeded(ictx, appclient.Shared, PlatformKey, u.SteamID64, fields.AvatarFullURL, st.SteamImageExpiryTime)
			icancel()
			if err == nil && res != nil {
				patch.ImageURL = res.PublicURL
				patch.AvatarPending = false
				patch.Error = ""
				steamLog.Info("avatar cached",
					slog.String("steamId", tailSteamID(u.SteamID64)),
					slog.String("url", res.PublicURL))
			} else {
				if err != nil {
					steamLog.Warn("avatar download failed",
						slog.String("steamId", tailSteamID(u.SteamID64)),
						slog.Any("err", err))
					patch.Error = err.Error()
				} else {
					steamLog.Warn("avatar download returned nil result",
						slog.String("steamId", tailSteamID(u.SteamID64)))
					patch.Error = "avatar download failed"
				}
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
	app.Event.Emit(AccountUpdatedEvent, p)
}

// GetSteamIDFormats exposes ID string conversions for the given SteamID64.
func (s *SteamService) GetSteamIDFormats(id64 string) (SteamIDFormats, error) {
	return FormatsFromID64(strings.TrimSpace(id64))
}

// SwapToSteamAccount switches to the given account (-1 uses OverrideState for persona in localconfig).
func (s *SteamService) SwapToSteamAccount(steamID64 string, personaState int, extraLaunchArgs []string) error {
	return SwapToAccount(strings.TrimSpace(steamID64), personaState, extraLaunchArgs)
}

// SteamAddNew clears saved login and launches Steam for a new account sign-in.
func (s *SteamService) SteamAddNew() error {
	return SwapToAccount("", -1, nil)
}

// LaunchSteam starts Steam without changing saved accounts.
func (s *SteamService) LaunchSteam() error {
	return LaunchSteamOnly(nil)
}

// ForgetSteamAccount removes an account row from loginusers.vdf and deletes cached avatar files.
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
	pj, err := platform.ResolvePlatformsJSONPath(exeDir)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(pj)
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
	pj, err := platform.ResolvePlatformsJSONPath(exeDir)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(pj)
	if err != nil {
		return "", err
	}
	return ResolveInstallFolder(exeDir, st, app, raw)
}

// GetInstalledGames lists installed titles (from appmanifest + optional Valve app list cache).
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

// OpenUserdataFolder opens userdata/<accountId> for this SteamID64.
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

// LoginAndLaunchGame swaps to the account then launches steam://rungameid/<appID>.
func (s *SteamService) LoginAndLaunchGame(steamID64 string, personaState int, appID string) error {
	steamID64 = strings.TrimSpace(steamID64)
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return errors.New("empty app id")
	}
	if err := SwapToAccount(steamID64, personaState, nil); err != nil {
		return err
	}
	url := "steam://rungameid/" + appID
	return winutil.Start("cmd.exe", []string{"/c", "start", "", url}, winutil.StartOpts{})
}

// ChangeAccountImage copies a local image into the Steam profile cache for this SteamID64.
func (s *SteamService) ChangeAccountImage(steamID64, sourcePath string) error {
	return profileimage.CacheLocalFile(PlatformKey, strings.TrimSpace(steamID64), strings.TrimSpace(sourcePath))
}
