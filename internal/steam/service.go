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
	"sync/atomic"
	"time"

	"TcNo-Acc-Switcher/internal/accountlist"
	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/basic"
	"TcNo-Acc-Switcher/internal/crashlog"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/profileimage"
	"TcNo-Acc-Switcher/internal/security"
	"TcNo-Acc-Switcher/internal/stats"
	"TcNo-Acc-Switcher/internal/steam/accountstore"
	"TcNo-Acc-Switcher/internal/winutil"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sync/semaphore"
)

const AccountUpdatedEvent = "steam-account-updated"

type AccountDTO struct {
	SteamID64 string `json:"steamId64"`

	PersonaName    string `json:"personaName"`
	AccountName    string `json:"accountName"`
	DisplayName    string `json:"displayName"`
	LastLogin      string `json:"lastLogin"`
	Offline        bool   `json:"offline"`
	ImageURL       string `json:"imageUrl"`
	StaticImageURL string `json:"staticImageUrl"`
	AvatarPending  bool   `json:"avatarPending"`
	MetaPending    bool   `json:"metaPending"`

	Vac bool `json:"vac"`
	Ltd bool `json:"ltd"`

	ShowSteamID     bool   `json:"showSteamId"`
	HasVisibleBan   bool   `json:"hasVisibleBan"`
	BanStatusHidden bool   `json:"banStatusHidden"`
	ShowVAC         bool   `json:"showVac"`
	ShowLimited     bool   `json:"showLimited"`
	ShowLastLogin   bool   `json:"showLastLogin"`
	ShowAccUsername bool   `json:"showAccUsername"`
	CollectInfo     bool   `json:"collectInfo"`
	ShowShortNotes  bool   `json:"showShortNotes"`
	Note            string `json:"note"`

	AvatarFrameURL string `json:"avatarFrameUrl"`
	// AvatarFrameStillURL is a single frame cut from an animated frame, shown
	// while animations are suspended. Empty when the frame does not animate.
	AvatarFrameStillURL string `json:"avatarFrameStillUrl"`
	MiniProfileHTML     string `json:"miniProfileHtml"`
	ShowMiniProfile     bool   `json:"showMiniProfile"`
	ShowAvatarFrame     bool   `json:"showAvatarFrame"`
	// ShowSteamGuardLock is the display setting, carried per row like the other
	// Show* flags. Whether an account HAS Steam Guard is hasSteamGuard.
	ShowSteamGuardLock bool `json:"showSteamGuardLock"`

	SyncError string `json:"syncError"`

	// CurrentSession: exactly one loginusers row has MostRecent=="1" and it is this account.
	CurrentSession bool `json:"currentSession"`

	Tags []basic.AccountTagDTO `json:"tags"`

	// ManualProfileImage: user-set avatar not replaced by refresh until removed.
	ManualProfileImage bool `json:"manualProfileImage"`

	// CS2CooldownExpiresAt is RFC3339 UTC, empty when there is no cooldown.
	CS2CooldownExpiresAt string `json:"cs2CooldownExpiresAt"`
	CS2CooldownPermanent bool   `json:"cs2CooldownPermanent"`
	ShowCS2Cooldown      bool   `json:"showCs2Cooldown"`
}

type AccountPatch struct {
	SteamID64 string `json:"steamId64"`

	ImageURL       string `json:"imageUrl"`
	StaticImageURL string `json:"staticImageUrl,omitempty"`
	Vac            bool   `json:"vac"`
	Ltd            bool   `json:"ltd"`

	AvatarPending bool `json:"avatarPending"`
	MetaPending   bool `json:"metaPending"`

	ManualProfileImage bool `json:"manualProfileImage,omitempty"`

	DisplayName string `json:"displayName,omitempty"`

	AvatarFrameURL      string `json:"avatarFrameUrl"`
	AvatarFrameStillURL string `json:"avatarFrameStillUrl"`
	MiniProfileHTML     string `json:"miniProfileHtml"`
	ShowMiniProfile     bool   `json:"showMiniProfile"`
	ShowAvatarFrame     bool   `json:"showAvatarFrame"`
	ShowSteamGuardLock  bool   `json:"showSteamGuardLock"`

	Error string `json:"error"`
}

type SteamService struct {
	mu sync.RWMutex

	refreshMu      sync.Mutex
	refreshRunning bool
	refreshQueued  bool
	refreshTimer   *time.Timer
	// refreshRetry re-runs a round that could not reach Steam; refreshFailures
	// is how many consecutive rounds have failed that way.
	refreshRetry    *time.Timer
	refreshFailures int
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

// resolveManualAvatarDisplay resolves the display fields for a user-set avatar.
// A leftover <id>_static is the Steam avatar from before the drop - the list
// build must not offer it as this avatar's static form - so the manual image is
// its own fallback, or none at all when it is a video.
//
// Every producer of a manual account's row has to go through this. The list
// build and the background refresh both write imageUrl/staticImageUrl for the
// same account, and any disagreement between them bumps the frontend's avatar
// epoch on every window focus, refetching the image and blanking the tile.
func resolveManualAvatarDisplay(manualURL string) (imageURL, fallbackStatic string) {
	return resolveSteamAvatarDisplay("", manualURL)
}

// steamAvatarPending takes its cache queries through a Lookup so a read-only
// caller can serve the whole list from one directory read, while a caller that
// downloads avatars as it goes keeps observing the live filesystem.
func steamAvatarPending(avatars profileimage.Lookup, steamID64, miniProfileHTML string, useMiniProfile bool, maxAgeDays int, isManual bool) bool {
	// A manual avatar is never re-downloaded - the refresh short-circuits to the
	// cached file and the expiry sweep skips it - so its age says nothing about
	// work still to come. Ageing one out marked it pending on every list build
	// while the refresh kept answering "not pending", and the two producers
	// flipped the flag against each other on every window focus.
	if isManual {
		return false
	}
	if useMiniProfile {
		if avatars.OlderThanDays(steamStaticAvatarID(steamID64), maxAgeDays) {
			return true
		}
		mediaSrc := ExtractMiniprofileAvatarMediaURL(miniProfileHTML)
		if mediaSrc == "" {
			return false
		}
		return avatars.OlderThanDays(steamID64, maxAgeDays)
	}
	return avatars.OlderThanDays(steamID64, maxAgeDays)
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
			imageURL, staticURL = resolveManualAvatarDisplay(u)
			return imageURL, staticURL, nil
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

	// An absent fragment is not the same statement as a fragment carrying no
	// animation. The first means the miniprofile could not be read at all - a
	// refusal, a deferral behind the rate limit, a cache that was just wiped - and
	// retiring a cached animation on the strength of it is exactly how one
	// transient 500 reset every tile to its static image. Only a fragment we
	// actually hold is allowed to answer the question.
	if strings.TrimSpace(miniProfileHTML) == "" {
		if cached, ok := profileimage.FindCached(PlatformKey, steamID64); ok {
			return cached, staticURL, nil
		}
		return staticURL, staticURL, nil
	}

	// No animated media: the full-quality static is the avatar. A plain-key
	// leftover — an old medium-quality copy or a since-removed animation —
	// would otherwise keep being found and shown instead of it.
	_ = profileimage.DeleteCached(PlatformKey, steamID64)
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := security.RequireUnlocked(); err != nil {
		return nil, err
	}
	list, err := s.GetSteamAccountsList()
	if err != nil {
		return nil, err
	}
	enrich, err := s.GetSteamAccountsEnrichment()
	if err != nil {
		return nil, err
	}
	out := accountlist.Merge(
		list,
		enrich,
		func(row SteamAccountListItemDTO) string { return row.SteamID64 },
		func(row SteamAccountEnrichmentDTO) string { return row.SteamID64 },
		mergeSteamAccountDTO,
	)
	syncSteamPlatformCounts(len(out))
	return out, nil
}

func (s *SteamService) SaveSteamAccountOrder(ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
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
	// The same union the list was built from, off the same root - both the list
	// and this count the whole union, so disagreeing on the root would reject
	// every reorder. Validating against loginusers.vdf alone would reject them
	// too, as soon as one account lived only in the store.
	users := knownAccountsForRoot(accountsRoot(root))
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	return LoadSettings()
}

func (s *SteamService) SaveSteamSettings(st Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := SaveSettings(st); err != nil {
		return err
	}
	// Turning a state off has to take its tags with it. They cannot be removed by
	// hand, so without this they would sit on the account permanently.
	stale := map[string]bool{
		basic.ManagedTagCS2Cooldown: !st.SteamCollectCS2Cooldowns,
		basic.ManagedTagCS2Prime:    !st.SteamShowCS2PrimeTag,
		basic.ManagedTagCS2NonPrime: !st.SteamShowCS2PrimeTag,
	}
	for tag, drop := range stale {
		if !drop {
			continue
		}
		if err := basic.ClearManagedTag(PlatformKey, tag); err != nil {
			steamLog.Warn("could not clear a managed CS2 tag",
				slog.String("tag", tag), slog.Any("err", err))
		}
	}
	return nil
}

func (s *SteamService) RefreshVACStatus() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	if err := ClearVACProfileCaches(); err != nil {
		return err
	}
	s.StartSteamProfileRefresh()
	return nil
}

// RefreshSteamGuardData asks the authenticated sweeps to re-read every vault
// account now: CS2 cooldown, rank and Prime, and the owned games library.
//
// Separate from RefreshAllSteamImages because nothing in this package can
// produce those figures - they need a signed-in session, which only the Steam
// Guard service holds. With the feature off there is simply nothing to sweep,
// so that is success, not an error to put in front of the user.
func (s *SteamService) RefreshSteamGuardData() error {
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	RequestSteamGuardSweep(true)
	return nil
}

func (s *SteamService) RefreshAllSteamImages() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	if err := ClearVACProfileCaches(); err != nil {
		return err
	}
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
	// Aged rather than deleted, so every tile keeps the face it already has until
	// its replacement has actually arrived.
	if err := profileimage.MarkAutomatedProfileCachesStale(PlatformKey); err != nil {
		return err
	}
	s.StartSteamProfileRefresh()
	return nil
}

// dropStaleMiniprofileFragment retires the cached miniprofile fragment once the
// assets it points at are due for replacement.
//
// It no longer deletes those assets. DownloadIfNeeded already re-fetches anything
// past its expiry and now retires the file it supersedes, so removing them here
// bought nothing - and it cost the list its faces for the whole of the round,
// which is precisely what a refresh looked like to the user: every tile blank,
// then slowly filling back in.
func dropStaleMiniprofileFragment(steamID64 string, maxAgeDays int) {
	ids := []string{
		steamID64,
		steamStaticAvatarID(steamID64),
		steamID64 + "_frame",
		steamID64 + "_nameplate",
		steamID64 + "_featuredbadge",
	}
	expired := deleteMiniprofileCacheIfOlder(steamID64, maxAgeDays)
	for _, id := range ids {
		if id == steamID64 && profileimage.HasManualProfileMarker(PlatformKey, steamID64) {
			continue
		}
		if p, ok := profileimage.CachedFilePath(PlatformKey, id); ok && profileimage.FileOlderThanDays(p, maxAgeDays) {
			expired = true
		}
	}
	if expired {
		deleteMiniprofileCache(steamID64)
	}
}

func (s *SteamService) StartSteamProfileRefresh() {
	if security.AppLocked() {
		return
	}
	s.refreshMu.Lock()
	if s.refreshTimer != nil {
		s.refreshTimer.Stop()
	}
	// A round starting now supersedes one that was only scheduled because the
	// last one failed.
	s.cancelProfileRefreshRetryLocked()
	s.refreshTimer = time.AfterFunc(500*time.Millisecond, func() {
		defer crashlog.Capture()
		s.runProfileRefresh()
	})
	s.refreshMu.Unlock()
}

// cancelProfileRefreshRetryLocked drops a scheduled retry. Callers hold refreshMu.
func (s *SteamService) cancelProfileRefreshRetryLocked() {
	if s.refreshRetry != nil {
		s.refreshRetry.Stop()
		s.refreshRetry = nil
	}
}

// profileRefreshQuiet reports whether an unreachable account should keep the
// tile it already has instead of replacing it with an error the next retry is
// about to clear.
func (s *SteamService) profileRefreshQuiet() bool {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	return s.refreshFailures < profileRefreshQuietFailures
}

// noteProfileRefreshOutcome records whether Steam answered this round and, when
// it did not, schedules the next one.
//
// Without this a failed round is terminal: every trigger for a refresh is an
// event elsewhere in the app (an account added, a setting changed, the list
// loading for the first time), and a machine sitting idle after a resume
// produces none of them.
func (s *SteamService) noteProfileRefreshOutcome(unreachable bool) {
	s.refreshMu.Lock()
	s.cancelProfileRefreshRetryLocked()
	if !unreachable {
		s.refreshFailures = 0
		s.refreshMu.Unlock()
		return
	}
	s.refreshFailures++
	failures := s.refreshFailures
	delay := profileRefreshRetryDelay(failures)
	s.refreshRetry = time.AfterFunc(delay, func() {
		defer crashlog.Capture()
		s.StartSteamProfileRefresh()
	})
	s.refreshMu.Unlock()
	steamLog.Info("profile refresh could not reach Steam; retrying",
		slog.Int("consecutiveFailures", failures), slog.Duration("in", delay))
}

// profileRefreshConcurrency is how many accounts are refreshed at once.
//
// Set from where the avatar CDN stops giving anything back. Measured against it:
// concurrency 5 returned 7.6 requests a second, 8 returned 10.2, and 12 returned
// 10.3 while median latency went from 711ms to 979ms. Past eight the extra
// requests only queue, so eight is the knee and there is nothing above it to win.
//
// Nothing else in the round objects: profile XML cleared 26 requests a second on
// its own, and the miniprofile endpoint - which does object, strongly - is held
// to its own budget by miniprofileLimiter rather than by throttling everything
// around it.
const profileRefreshConcurrency = 8

func (s *SteamService) runProfileRefresh() {
	if security.AppLocked() {
		return
	}
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
	users := knownAccountsForRoot(accountsRoot(root))
	if len(users) == 0 {
		steamLog.Warn("no Steam accounts to refresh")
		return
	}

	steamLog.Info("refreshing Steam profiles",
		slog.Int("accounts", len(users)), slog.Int("concurrency", profileRefreshConcurrency))

	vacRows, _ := LoadVacCache(st.SteamImageExpiryTime)
	vm := vacMap(vacRows)
	var vmMu sync.Mutex

	ctx := context.Background()
	sem := semaphore.NewWeighted(profileRefreshConcurrency)
	var wg sync.WaitGroup
	// One verdict for the whole round: every account is talking to the same
	// Steam over the same adapter, so if any of them could not reach it the
	// round is worth repeating.
	var unreachable atomic.Bool
	quiet := s.profileRefreshQuiet()

	for _, u := range users {
		u := u
		wg.Add(1)
		go func() {
			defer crashlog.Capture()
			defer wg.Done()
			_ = sem.Acquire(ctx, 1)
			defer sem.Release(1)

			dropStaleMiniprofileFragment(u.SteamID64, st.SteamImageExpiryTime)

			vmMu.Lock()
			prev := vm[u.SteamID64]
			vmMu.Unlock()

			patch := AccountPatch{
				SteamID64:          u.SteamID64,
				Vac:                prev.Vac,
				Ltd:                prev.Ltd,
				ShowMiniProfile:    st.SteamShowMiniProfile,
				ShowAvatarFrame:    st.SteamShowAvatarFrame,
				ShowSteamGuardLock: st.SteamShowSteamGuardLock,
			}

			fields, err := fetchProfileXMLWithRetry(
				ctx,
				defaultProfileXMLRetryPolicy,
				func(attemptCtx context.Context) (ProfileXMLFields, error) {
					return FetchProfileXML(attemptCtx, appclient.Shared, u.SteamID64)
				},
				func(retryErr error) {
					patch.Error, patch.MetaPending = profileRefreshErrorState(retryErr, true)
					patch.AvatarPending = false
					s.emit(patch)
				},
			)

			if err != nil {
				steamLog.Warn("community profile XML failed",
					slog.String("steamId", tailSteamID(u.SteamID64)),
					slog.Any("err", err))
				if isTransientProfileRefreshError(err) {
					unreachable.Store(true)
					if quiet {
						patch.Error, patch.MetaPending = "", true
						patch.AvatarPending = false
						s.emit(patch)
						return
					}
				}
				patch.Error, patch.MetaPending = profileRefreshErrorState(err, false)
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
			patch.ShowSteamGuardLock = st.SteamShowSteamGuardLock

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
					// The last good fragment is kept rather than left empty. A
					// refusal says nothing about the account, but an empty fragment
					// says something quite specific further down - it is how
					// "this account has no animated avatar" is expressed - and one
					// transient 500 from a rate limit was enough to delete the
					// cached animation and reset every tile to its static image.
					patch.MiniProfileHTML = ReadCachedMiniprofileHTML(u.SteamID64)
				} else {
					patch.MiniProfileHTML = miniHTML
					if n := ExtractMiniprofileDisplayName(miniHTML); n != "" {
						patch.DisplayName = n
					}
					if st.SteamShowAvatarFrame && strings.HasPrefix(strings.TrimSpace(frameSrc), "/img/") {
						// The miniprofile embed already serves the frame from the app's
						// own origin; there is nothing left to download.
						patch.AvatarFrameURL = strings.TrimSpace(frameSrc)
					} else if st.SteamShowAvatarFrame && strings.TrimSpace(frameSrc) != "" {
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
					// However the frame arrived, an animated one needs a still cut
					// from it: the page cannot freeze an APNG, and the only frame it
					// can reach on its own is the loudest one.
					if patch.AvatarFrameURL != "" {
						patch.AvatarFrameStillURL = profileimage.EnsureAnimatedStill(
							PlatformKey, u.SteamID64+"_frame")
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
					patch.ImageURL, patch.StaticImageURL = resolveManualAvatarDisplay(cachedURL)
					patch.AvatarPending = false
					patch.Error = ""
					s.emit(patch)
					return
				}
			}

			// Live lookups, not a snapshot: this path downloads avatars as it
			// runs, so it has to see what it just wrote.
			if !steamAvatarPending(profileimage.DirectLookup(PlatformKey), u.SteamID64, patch.MiniProfileHTML, useMiniProfile, st.SteamImageExpiryTime, false) {
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
				// Same treatment as the profile fetch above, and for the same
				// reason: this is the avatar CDN being unreachable, not
				// anything the user can read a Go transport error about.
				transient := isTransientProfileRefreshError(err)
				if transient {
					unreachable.Store(true)
				}
				patch.Error = ""
				if !transient || !quiet {
					patch.Error, _ = profileRefreshErrorState(err, false)
				}
				patch.AvatarPending = false
			}
			s.emit(patch)
		}()
	}
	wg.Wait()
	s.noteProfileRefreshOutcome(unreachable.Load())

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
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	return SwapToAccount(strings.TrimSpace(steamID64), personaState, extraLaunchArgs)
}

func (s *SteamService) SteamAddNew() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	return SwapToAccount("", -1, nil)
}

func (s *SteamService) LaunchSteam() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	return LaunchSteamOnly(nil)
}

func (s *SteamService) ForgetSteamAccount(steamID64 string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
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
	// Steam Guard first, because its index rebuilds a row of its own: an account
	// forgotten while a record survives comes back on the next refresh, nameless,
	// since the login name went with the vdf row. This also refuses outright for
	// an authenticator, whose secrets cannot be recreated.
	if err := releaseSteamGuardRecord(steamID64); err != nil {
		return err
	}
	// The store next: it is now the thing that decides whether the account is
	// listed, so a failure here has to abort before Steam's file is touched.
	// Otherwise the row would come back on the next refresh and look like the
	// Forget silently did nothing.
	if err := accountstore.Remove(steamID64); err != nil {
		return err
	}
	if err := RemoveSteamAccountFromVDF(root, steamID64); err != nil {
		return err
	}
	if err := removeFromOrder(steamID64); err != nil {
		steamLog.Warn("could not prune the forgotten account from the saved order",
			slog.String("steamId", tailSteamID(steamID64)), slog.Any("err", err))
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
	return installRoot()
}

func (s *SteamService) GetInstalledGames() ([]InstalledGameInfo, error) {
	if err := security.RequireUnlocked(); err != nil {
		return nil, err
	}
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
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
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
	if err := winutil.Start("cmd.exe", []string{"/c", "start", "", url}, winutil.StartOpts{HideWindow: true}); err != nil {
		return err
	}
	_ = stats.IncrementGamesLaunched(PlatformKey)
	return nil
}

func (s *SteamService) ChangeAccountImage(steamID64, sourcePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
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
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
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

// SetBanStatusHidden adds or removes one account from the list whose VAC or
// limited status is not shown. The ban itself is untouched - this only stops
// the switcher painting it on every visit, which is a display preference, not
// an attempt to make the account look clean.
func (s *SteamService) SetBanStatusHidden(steamID64 string, hidden bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := security.RequireUnlocked(); err != nil {
		return err
	}
	steamID64 = strings.TrimSpace(steamID64)
	if steamID64 == "" {
		return errors.New("empty steam id")
	}
	return UpdateSettings(func(st *Settings) error {
		kept := make([]string, 0, len(st.HiddenBanStatus)+1)
		for _, id := range st.HiddenBanStatus {
			if id = strings.TrimSpace(id); id != "" && id != steamID64 {
				kept = append(kept, id)
			}
		}
		if hidden {
			kept = append(kept, steamID64)
		}
		st.HiddenBanStatus = kept
		return nil
	})
}
