package basic

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"TcNo-Acc-Switcher/internal/appclient"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/profileimage"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sync/semaphore"
)

// AccountImageUpdatedEvent is emitted when a remote profile image finishes downloading (or is cleared).
const AccountImageUpdatedEvent = "basic-account-image-updated"

// AccountImagePatch updates one row on the generic platform account list.
type AccountImagePatch struct {
	PlatformKey   string `json:"platformKey"`
	UniqueID      string `json:"uniqueId"`
	ImageURL      string `json:"imageUrl"`
	AvatarPending bool   `json:"avatarPending"`
}

var basicProfileImgLog = slog.Default().With("component", "basic-profile-image")

func emitAccountImagePatch(p AccountImagePatch) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(AccountImageUpdatedEvent, p)
}

// PlatformUsesRemoteProfileImages reports whether this platform has any supported profile image source.
func (b *BasicService) PlatformUsesRemoteProfileImages(platformKey string) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	d, _, err := readDescriptor(strings.TrimSpace(platformKey))
	if err != nil {
		return false, err
	}
	tpl := strings.TrimSpace(d.Extras.ProfilePicPath)
	remoteTpl := remoteProfilePicTemplate(tpl) && !strings.Contains(tpl, "%LARGEST%")
	return remoteTpl || platformHasProfileImageSource(platformKey), nil
}

// StartBasicProfileImageRefresh downloads missing or stale remote profile images in the background (bounded concurrency).
func (b *BasicService) StartBasicProfileImageRefresh(platformKey string) {
	go b.runProfileImageRefresh(strings.TrimSpace(platformKey))
}

// RefreshAllBasicProfileImages deletes cached remote profile images for every known account, then starts a background re-download (like Steam "Refresh images").
func (b *BasicService) RefreshAllBasicProfileImages(platformKey string) error {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" {
		return nil
	}
	f, err := readIdsFile(platformKey)
	if err != nil {
		return err
	}
	for uid := range f.IDs {
		uid = strings.TrimSpace(uid)
		if uid == "" {
			continue
		}
		_ = profileimage.DeleteCached(platformKey, uid)
	}
	go b.runProfileImageRefresh(platformKey)
	return nil
}

// ClearAllBasicProfileImages deletes all cached profile images for this platform.
func (b *BasicService) ClearAllBasicProfileImages(platformKey string) error {
	return profileimage.DeletePlatformCached(strings.TrimSpace(platformKey))
}

// PlatformProfileImagesSavedPerAccount reports whether profile image source files are saved in account LoginFiles.
func (b *BasicService) PlatformProfileImagesSavedPerAccount(platformKey string) bool {
	return platformProfileImagesSavedPerAccount(platformKey)
}

// RefreshSavedBasicProfileImages clears and requeues profile image downloads from account-saved source files.
func (b *BasicService) RefreshSavedBasicProfileImages(platformKey string) error {
	platformKey = strings.TrimSpace(platformKey)
	if platformKey == "" || !platformProfileImagesSavedPerAccount(platformKey) {
		return nil
	}
	if err := profileimage.DeletePlatformCached(platformKey); err != nil {
		return err
	}
	f, err := readIdsFile(platformKey)
	if err != nil {
		return err
	}
	for uid, accountName := range f.IDs {
		uid = strings.TrimSpace(uid)
		accountName = strings.TrimSpace(accountName)
		if uid == "" || accountName == "" {
			continue
		}
		src, ok, err := platformProfileImageSourceFromSavedAccount(platformKey, accountName)
		if err != nil || !ok {
			continue
		}
		if strings.TrimSpace(src.LocalPath) != "" {
			queueProfileImageLocalCache(platformKey, uid, src.LocalPath)
		} else if strings.TrimSpace(src.RemoteURL) != "" {
			queueProfileImageDownload(platformKey, uid, src.RemoteURL, 0)
		}
	}
	return nil
}

func (b *BasicService) runProfileImageRefresh(platformKey string) {
	b.imgRefreshMu.Lock()
	if b.imgRefreshRunning {
		b.imgRefreshQueued = true
		b.imgRefreshMu.Unlock()
		basicProfileImgLog.Debug("profile image refresh coalesced: already running")
		return
	}
	b.imgRefreshRunning = true
	b.imgRefreshMu.Unlock()

	defer func() {
		var again bool
		b.imgRefreshMu.Lock()
		b.imgRefreshRunning = false
		again = b.imgRefreshQueued
		b.imgRefreshQueued = false
		b.imgRefreshMu.Unlock()
		if again {
			go b.runProfileImageRefresh(platformKey)
		}
	}()

	d, _, err := readDescriptor(platformKey)
	if err != nil {
		basicProfileImgLog.Warn("readDescriptor failed", slog.String("platform", platformKey), slog.Any("err", err))
		return
	}
	folder, _ := resolveExeFolder(b.deps(), platformKey)
	baseCtx := platform.PathTokenContext{PlatformFolder: folder}
	vars := resolveDescriptorVariables(d, folder, baseCtx, "", false)
	tpl := strings.TrimSpace(expandDescriptorVariables(d.Extras.ProfilePicPath, vars))
	remoteTpl := remoteProfilePicTemplate(tpl) && !strings.Contains(tpl, "%LARGEST%")

	exeDir, err := platform.ResolveExeDir()
	if err != nil {
		basicProfileImgLog.Warn("ResolveExeDir failed", slog.Any("err", err))
		return
	}
	appSt, err := platform.LoadAppSettings(exeDir)
	if err != nil {
		basicProfileImgLog.Warn("LoadAppSettings failed", slog.Any("err", err))
		return
	}
	if appSt.OfflineMode {
		basicProfileImgLog.Info("profile image refresh skipped: offline mode")
		return
	}

	ps, err := platform.LoadPlatformSettings(platformKey)
	if err != nil {
		basicProfileImgLog.Warn("LoadPlatformSettings failed", slog.Any("err", err))
		return
	}
	maxAge := ps.ProfileImageExpiryDays
	if maxAge <= 0 {
		maxAge = 7
	}
	if !ps.PullAccountImagesOnSwitch {
		return
	}
	if platformProfileImagesSavedPerAccount(platformKey) {
		b.queueMissingSavedProfileImages(platformKey)
	}
	if !remoteTpl {
		return
	}

	f, err := readIdsFile(platformKey)
	if err != nil {
		basicProfileImgLog.Warn("readIdsFile failed", slog.Any("err", err))
		return
	}
	var uids []string
	for uid := range f.IDs {
		uid = strings.TrimSpace(uid)
		if uid != "" {
			uids = append(uids, uid)
		}
	}
	if len(uids) == 0 {
		return
	}

	basicProfileImgLog.Info("background profile image refresh", slog.String("platform", platformKey), slog.Int("accounts", len(uids)), slog.Int("concurrency", 5))

	ctx := context.Background()
	sem := semaphore.NewWeighted(5)
	var wg sync.WaitGroup

	for _, uid := range uids {
		uid := uid
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = sem.Acquire(ctx, 1)
			defer sem.Release(1)

			ctx2 := platform.PathTokenContext{PlatformFolder: folder, UniqueID: uid}
			url, err := profilePicPathResolved(tpl, folder, ctx2)
			if err != nil || strings.TrimSpace(url) == "" || !remoteProfilePicTemplate(url) {
				emitAccountImagePatch(AccountImagePatch{
					PlatformKey: platformKey, UniqueID: uid, ImageURL: "", AvatarPending: false,
				})
				return
			}

			ictx, cancel := context.WithTimeout(ctx, 30*time.Second)
			res, err := profileimage.DownloadIfNeeded(ictx, appclient.Shared, platformKey, uid, url, maxAge)
			cancel()

			patch := AccountImagePatch{PlatformKey: platformKey, UniqueID: uid, AvatarPending: false}
			if err == nil && res != nil {
				patch.ImageURL = res.PublicURL
			} else if u, ok := profileimage.FindCached(platformKey, uid); ok {
				patch.ImageURL = u
			}
			emitAccountImagePatch(patch)
		}()
	}
	wg.Wait()
	basicProfileImgLog.Info("profile image refresh finished", slog.String("platform", platformKey))
}

func (b *BasicService) queueMissingSavedProfileImages(platformKey string) {
	f, err := readIdsFile(platformKey)
	if err != nil {
		return
	}
	for uid, accountName := range f.IDs {
		uid = strings.TrimSpace(uid)
		accountName = strings.TrimSpace(accountName)
		if uid == "" || accountName == "" {
			continue
		}
		if _, ok := profileimage.FindCached(platformKey, uid); ok {
			continue
		}
		src, ok, err := platformProfileImageSourceFromSavedAccount(platformKey, accountName)
		if err != nil || !ok {
			continue
		}
		if strings.TrimSpace(src.LocalPath) != "" {
			queueProfileImageLocalCache(platformKey, uid, src.LocalPath)
		} else if strings.TrimSpace(src.RemoteURL) != "" {
			queueProfileImageDownload(platformKey, uid, src.RemoteURL, 0)
		}
	}
}
