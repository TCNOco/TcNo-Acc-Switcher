package steamguard

import (
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/pixelconv"
	"TcNo-Acc-Switcher/internal/steam"
	"TcNo-Acc-Switcher/internal/steamguard/capability"
	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/protocol"
	"TcNo-Acc-Switcher/internal/steamguard/qr"
	"TcNo-Acc-Switcher/internal/steamguard/qrattempt"
	"TcNo-Acc-Switcher/internal/steamguard/qrcapture"
	"TcNo-Acc-Switcher/internal/steamguard/qrimage"
	"TcNo-Acc-Switcher/internal/steamguard/qrregion"
	"TcNo-Acc-Switcher/internal/steamguard/vault"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const qrAttemptLifetime = 2 * time.Minute

const qrApprovalTimeout = 15 * time.Second

var ErrSteamMobileSession = errors.New("Steam mobile session is unavailable; log in again")

type QRScanState string

const (
	QRScanReady         QRScanState = "ready"
	QRScanNoCode        QRScanState = "no-code"
	QRScanMultipleCodes QRScanState = "multiple-codes"
	QRScanSteamNotFound QRScanState = "steam-not-found"
	QRScanNoWindow      QRScanState = "no-window"
	QRScanUnavailable   QRScanState = "unavailable"
	QRScanInvalidImage  QRScanState = "invalid-image"
	QRScanWorkLimit     QRScanState = "work-limit"
	QRScanCanceled      QRScanState = "canceled"
	QRScanBusy          QRScanState = "busy"
	QRScanUnsupported   QRScanState = "unsupported"
	QRScanCaptureFailed QRScanState = "capture-failed"
)

type QRScanResult struct {
	State          QRScanState `json:"state"`
	Attempt        string      `json:"attempt,omitempty"`
	CandidateCount int         `json:"candidateCount,omitempty"`
}

type steamQRScanner interface {
	Discover(string) (qrcapture.Discovery, error)
	CaptureWindow(qrcapture.Candidate) (qrcapture.Capture, error)
}

type steamQRRegionSelector interface {
	Select(context.Context) (qrregion.Frame, error)
}

type steamQRAuthenticator interface {
	GetAuthSessionInfo(context.Context, protocol.AuthSessionInfoRequest, time.Duration) (protocol.AuthSessionInfo, error)
	UpdateAuthSessionWithMobileConfirmation(context.Context, protocol.MobileConfirmationRequest, time.Duration) (protocol.ChallengeResult, error)
}

type QRApprovalView struct {
	AccountName              string `json:"accountName"`
	DeviceName               string `json:"deviceName,omitempty"`
	IPAddress                string `json:"ipAddress,omitempty"`
	Location                 string `json:"location,omitempty"`
	Platform                 string `json:"platform"`
	Application              string `json:"application"`
	Persistence              string `json:"persistence"`
	LocationMismatch         bool   `json:"locationMismatch"`
	HighUsageLogin           bool   `json:"highUsageLogin"`
	PreviouslyUsedLocation   bool   `json:"previouslyUsedLocation"`
	RequestorDeviceTrustCode int32  `json:"requestorDeviceTrustCode,omitempty"`
}

// qrLogger is the component logger for the QR login flow. It records the failing
// step and the resulting state only: QR payloads, challenges, signatures and
// mobile tokens never reach it.
func qrLogger() *slog.Logger {
	return slog.Default().With("component", "steamguard.qr")
}

// logQRFailure records the underlying error before it collapses into a
// QRScanState the UI can render.
// logQRAttempt records what the decoder was actually handed.
//
// A scan that finds nothing says only "no-code", which is the same answer for a
// photograph too blurred to read and for a capture that grabbed sixty pixels
// because the screen is scaled. The size tells those apart, and it is the one
// thing about the image that is safe to write down.
func logQRAttempt(step, accountID string, width, height int) {
	qrLogger().Debug("Steam QR decode attempt",
		"step", step, "steamId64", accountID, "width", width, "height", height)
}

func logQRFailure(step, accountID string, state QRScanState, err error) {
	attributes := []any{"step", step, "steamId64", accountID, "state", string(state)}
	if err != nil {
		attributes = append(attributes, "error", err)
	}
	qrLogger().Warn("Steam QR scan step failed", attributes...)
}

func resolveSteamExecutable() (string, bool) { return steam.ResolveSteamExePath() }

// PickQRScreenshot opens a native single-file picker. Image bytes remain in Go.
func (s *Service) PickQRScreenshot() (string, error) {
	app := application.Get()
	if app == nil {
		return "", errors.New("application not initialised")
	}
	dialog := app.Dialog.OpenFile().
		SetTitle("Choose a Steam login QR screenshot").
		AddFilter("PNG and JPEG images", "*.png;*.jpg;*.jpeg")
	if owner := dialogOwnerWindow(); owner != nil {
		dialog = dialog.AttachToWindow(owner)
	}
	selected, err := dialog.PromptForSingleSelection()
	logDialogOutcome("pick-qr-screenshot", strings.TrimSpace(selected) != "", err)
	if err != nil {
		if dialogCancelled(err) {
			// Cancel is a clean outcome: an empty path with no error.
			return "", nil
		}
		return "", err
	}
	selected = strings.TrimSpace(selected)
	if selected == "" {
		return "", nil
	}
	if !filepath.IsAbs(selected) {
		return "", qrimage.ErrUnsafePath
	}
	extension := strings.ToLower(filepath.Ext(selected))
	if extension != ".png" && extension != ".jpg" && extension != ".jpeg" {
		return "", qrimage.ErrUnsupportedImage
	}
	return selected, nil
}

// DecodeQRScreenshot validates and decodes one user-selected image without
// returning the QR payload to the WebView.
func (s *Service) DecodeQRScreenshot(accountID, path, token string) (QRScanResult, error) {
	binding, err := s.authorizeQRBinding(accountID, token)
	if err != nil {
		return QRScanResult{}, err
	}
	if err := s.revokeQRAttempt(binding.AccountID); err != nil {
		return QRScanResult{}, err
	}
	frame, err := qrimage.Load(strings.TrimSpace(path))
	if err != nil {
		logQRFailure("load", binding.AccountID, QRScanInvalidImage, err)
		return QRScanResult{State: QRScanInvalidImage}, nil
	}
	logQRAttempt("screenshot", binding.AccountID, frame.Width, frame.Height)
	ctx, cancel := context.WithTimeout(context.Background(), qrimage.MaxCandidateDecodeTime+time.Second)
	candidates, err := qrimage.DecodeCandidates(ctx, frame)
	cancel()
	if err != nil {
		result := decodeFailureResult(err)
		logQRFailure("decode", binding.AccountID, result.State, err)
		return result, nil
	}
	return s.finishQRScan("screenshot", accountID, token, binding, candidates)
}

// ScanSteamQR discovers verified Steam windows and scans only their visible,
// monitor-clipped rectangles.
func (s *Service) ScanSteamQR(accountID, token string) (QRScanResult, error) {
	binding, err := s.authorizeQRBinding(accountID, token)
	if err != nil {
		return QRScanResult{}, err
	}
	if err := s.revokeQRAttempt(binding.AccountID); err != nil {
		return QRScanResult{}, err
	}
	if s.qrScanner == nil {
		logQRFailure("discover", binding.AccountID, QRScanUnavailable, nil)
		return QRScanResult{State: QRScanUnavailable}, nil
	}
	configuredPath := ""
	if s.resolveSteamExecutableFn != nil {
		configuredPath, _ = s.resolveSteamExecutableFn()
	}
	discovery, err := s.qrScanner.Discover(configuredPath)
	if err != nil {
		logQRFailure("discover", binding.AccountID, QRScanUnavailable, err)
		return QRScanResult{State: QRScanUnavailable}, nil
	}
	switch discovery.State {
	case qrcapture.DiscoverySteamNotFound:
		logQRFailure("discover", binding.AccountID, QRScanSteamNotFound, nil)
		return QRScanResult{State: QRScanSteamNotFound}, nil
	case qrcapture.DiscoveryNoWindow:
		logQRFailure("discover", binding.AccountID, QRScanNoWindow, nil)
		return QRScanResult{State: QRScanNoWindow}, nil
	case qrcapture.DiscoveryReady:
	default:
		logQRFailure("discover", binding.AccountID, QRScanUnavailable, nil)
		return QRScanResult{State: QRScanUnavailable}, nil
	}

	frames, captured := s.captureQRFrames(discovery.Windows)
	if len(frames) == 0 {
		if captured {
			logQRFailure("capture", binding.AccountID, QRScanNoCode, nil)
			return QRScanResult{State: QRScanNoCode}, nil
		}
		logQRFailure("capture", binding.AccountID, QRScanUnavailable, nil)
		return QRScanResult{State: QRScanUnavailable}, nil
	}
	for _, captured := range frames {
		logQRAttempt("capture", binding.AccountID, captured.Width, captured.Height)
	}
	ctx, cancel := context.WithTimeout(context.Background(), qrimage.MaxCandidateDecodeTime+time.Second)
	candidates, decodeErr := qrimage.DecodeCandidates(ctx, frames...)
	cancel()
	if decodeErr != nil {
		result := decodeFailureResult(decodeErr)
		logQRFailure("decode", binding.AccountID, result.State, decodeErr)
		return result, nil
	}
	return s.finishQRScan("window", accountID, token, binding, candidates)
}

// SelectQRRegion displays the native region overlay, captures only the chosen
// pixels in memory, and binds the operation to the active sensitive-view lease.
func (s *Service) SelectQRRegion(accountID, token string) (QRScanResult, error) {
	original, err := s.authorizeQRBinding(accountID, token)
	if err != nil {
		return QRScanResult{}, err
	}
	if err := s.revokeQRAttempt(original.AccountID); err != nil {
		return QRScanResult{}, err
	}
	if s.qrRegionSelector == nil {
		logQRFailure("region", original.AccountID, QRScanUnsupported, nil)
		return QRScanResult{State: QRScanUnsupported}, nil
	}

	capabilityBinding, err := s.resolveQRCapabilityBinding(accountID, token, original.VaultGeneration)
	if err != nil {
		return QRScanResult{}, err
	}
	ctx, cancel, operation, err := s.beginQRRegionSelection(capabilityBinding)
	if errors.Is(err, qrregion.ErrBusy) {
		logQRFailure("region", original.AccountID, QRScanBusy, err)
		return QRScanResult{State: QRScanBusy}, nil
	}
	if err != nil {
		return QRScanResult{}, err
	}
	defer func() {
		cancel()
		s.finishQRRegionSelection(operation)
	}()

	frame, err := s.qrRegionSelector.Select(ctx)
	defer frame.Wipe()
	if err != nil {
		result := qrRegionFailureResult(err)
		logQRFailure("region", original.AccountID, result.State, err)
		return result, nil
	}
	normalized, ok := normalizeBGRAFrame(frame.Width, frame.Height, frame.Stride, frame.BGRA)
	if !ok {
		logQRFailure("region", original.AccountID, QRScanCaptureFailed, nil)
		return QRScanResult{State: QRScanCaptureFailed}, nil
	}
	logQRAttempt("region", original.AccountID, normalized.Width, normalized.Height)
	decodeCtx, decodeCancel := context.WithTimeout(ctx, qrimage.MaxCandidateDecodeTime+time.Second)
	candidates, decodeErr := qrimage.DecodeCandidates(decodeCtx, normalized)
	decodeCancel()
	if decodeErr != nil {
		if errors.Is(decodeErr, context.Canceled) {
			logQRFailure("decode", original.AccountID, QRScanCanceled, decodeErr)
			return QRScanResult{State: QRScanCanceled}, nil
		}
		result := decodeFailureResult(decodeErr)
		logQRFailure("decode", original.AccountID, result.State, decodeErr)
		return result, nil
	}
	return s.finishQRScan("region", accountID, token, original, candidates)
}

// CancelQRRegion cancels the current selection only when it belongs to the
// supplied account and sensitive-view capability.
func (s *Service) CancelQRRegion(accountID, token string) error {
	original, err := s.authorizeQRBinding(accountID, token)
	if err != nil {
		return err
	}
	binding, err := s.resolveQRCapabilityBinding(accountID, token, original.VaultGeneration)
	if err != nil {
		return err
	}
	s.cancelQRRegionSelection(binding.LeaseID)
	return nil
}

// GetQRApproval returns bounded requestor details without consuming the
// single-use attempt. The challenge and mobile token remain in Go.
func (s *Service) GetQRApproval(accountID, attempt, token string) (QRApprovalView, error) {
	binding, account, err := s.authorizeQRAccount(accountID, token)
	if err != nil {
		return QRApprovalView{}, err
	}
	accessToken, err := mobileAccessToken(account, binding.AccountID)
	if err != nil {
		return QRApprovalView{}, err
	}
	var info protocol.AuthSessionInfo
	err = s.qrAttempts.Inspect(qrattempt.ID(strings.TrimSpace(attempt)), binding, func(payload []byte) error {
		challenge, parseErr := qr.ParseChallenge(string(payload))
		if parseErr != nil {
			return parseErr
		}
		ctx, cancel := context.WithTimeout(context.Background(), qrApprovalTimeout)
		defer cancel()
		info, parseErr = s.qrAuth.GetAuthSessionInfo(ctx, protocol.AuthSessionInfoRequest{
			AccessToken: accessToken,
			ClientID:    challenge.ClientID,
		}, qrApprovalTimeout)
		return parseErr
	})
	if err != nil {
		logQRFailure("approval-info", binding.AccountID, "", err)
		return QRApprovalView{}, err
	}
	// Same as finishQRScan: the account must still be the one the inspection ran
	// against, but a generation carried forward by a session-token sweep is not a
	// reason to refuse an approval the user is looking at.
	current, err := s.authorizeQRBinding(accountID, token)
	if err != nil || current.AccountID != binding.AccountID {
		if err != nil {
			return QRApprovalView{}, err
		}
		return QRApprovalView{}, capability.ErrInvalidCapability
	}
	return qrApprovalView(account.AccountName, info), nil
}

// AuthorizeQRLogin consumes the attempt before signing and submitting approval.
// A network failure requires a fresh scan so a challenge is never replayed.
func (s *Service) AuthorizeQRLogin(accountID, attempt, token string) error {
	binding, account, err := s.authorizeQRAccount(accountID, token)
	if err != nil {
		return err
	}
	accessToken, err := mobileAccessToken(account, binding.AccountID)
	if err != nil {
		return err
	}
	steamID, err := strconv.ParseUint(binding.AccountID, 10, 64)
	if err != nil || steamID == 0 {
		return ErrAccountNotFound
	}
	sharedSecret, err := base64.StdEncoding.Strict().DecodeString(account.SharedSecret)
	if err != nil || len(sharedSecret) != 20 {
		wipe(sharedSecret)
		return ErrSteamMobileSession
	}
	defer wipe(sharedSecret)

	approvalErr := s.qrAttempts.Consume(qrattempt.ID(strings.TrimSpace(attempt)), binding, func(payload []byte) error {
		challenge, parseErr := qr.ParseChallenge(string(payload))
		if parseErr != nil {
			return parseErr
		}
		ctx, cancel := context.WithTimeout(context.Background(), qrApprovalTimeout)
		defer cancel()
		info, requestErr := s.qrAuth.GetAuthSessionInfo(ctx, protocol.AuthSessionInfoRequest{
			AccessToken: accessToken,
			ClientID:    challenge.ClientID,
		}, qrApprovalTimeout)
		if requestErr != nil {
			return requestErr
		}
		signature, signErr := qr.SignChallenge(challenge, steamID, sharedSecret)
		if signErr != nil {
			return signErr
		}
		defer wipe(signature)
		result, requestErr := s.qrAuth.UpdateAuthSessionWithMobileConfirmation(ctx, protocol.MobileConfirmationRequest{
			AccessToken: accessToken,
			Version:     int32(challenge.Version),
			ClientID:    challenge.ClientID,
			SteamID:     steamID,
			Signature:   signature,
			Confirm:     true,
			Persistence: info.RequestedPersistence,
		}, qrApprovalTimeout)
		if requestErr != nil {
			return requestErr
		}
		if result.State != protocol.AuthResultChallengeAccepted {
			return ErrSteamMobileSession
		}
		return nil
	})
	if approvalErr != nil {
		logQRFailure("approve", binding.AccountID, "", approvalErr)
		return approvalErr
	}
	qrLogger().Info("Steam QR login approved", "steamId64", binding.AccountID)
	return nil
}

func (s *Service) DismissQRLogin(accountID, attempt, token string) error {
	binding, err := s.authorizeQRBinding(accountID, token)
	if err != nil {
		return err
	}
	if err := s.qrAttempts.Inspect(qrattempt.ID(strings.TrimSpace(attempt)), binding, func([]byte) error { return nil }); err != nil {
		return err
	}
	return s.qrAttempts.RevokeAccount(binding.AccountID)
}

func (s *Service) captureQRFrames(windows []qrcapture.Candidate) ([]*qrimage.Frame, bool) {
	frames := make([]*qrimage.Frame, 0, qrimage.MaxCandidateFrames)
	captured := false
	for _, window := range windows {
		if len(frames) >= qrimage.MaxCandidateFrames {
			break
		}
		capture, err := s.qrScanner.CaptureWindow(window)
		if err != nil || capture.State != qrcapture.CaptureReady {
			capture.Wipe()
			continue
		}
		captured = true
		for i := range capture.Frames {
			if len(frames) >= qrimage.MaxCandidateFrames {
				break
			}
			frame, ok := normalizeCapturedFrame(capture.Frames[i])
			if ok {
				frames = append(frames, frame)
			}
		}
		capture.Wipe()
	}
	return frames, captured
}

func normalizeCapturedFrame(source qrcapture.Frame) (*qrimage.Frame, bool) {
	return normalizeBGRAFrame(source.Width, source.Height, source.Stride, source.BGRA)
}

func normalizeBGRAFrame(width, height, stride int, source []byte) (*qrimage.Frame, bool) {
	if width <= 0 || height <= 0 || stride != width*4 ||
		int64(width)*int64(height) > qrimage.MaxCandidatePixels ||
		len(source) != stride*height {
		return nil, false
	}
	destination := &qrimage.Frame{
		Width:  width,
		Height: height,
		Stride: stride,
		Pixels: make([]byte, len(source)),
	}
	// GDI leaves the capture's alpha byte undefined, so it is forced opaque
	// rather than carried through.
	pixelconv.BGRAToNRGBAOpaque(destination.Pixels, source)
	return destination, true
}

func (s *Service) resolveQRCapabilityBinding(accountID, token, generation string) (capability.Binding, error) {
	if s.capabilities == nil {
		return capability.Binding{}, capability.ErrInvalidCapability
	}
	binding, err := s.capabilities.Resolve(token)
	if err != nil || binding.WindowName != mainWindowName || binding.Scope != modalCapabilityScope ||
		binding.AccountID != strings.TrimSpace(accountID) || binding.VaultGeneration != generation {
		return capability.Binding{}, capability.ErrInvalidCapability
	}
	return binding, nil
}

func (s *Service) beginQRRegionSelection(binding capability.Binding) (context.Context, context.CancelFunc, uint64, error) {
	s.contentProtectionMu.Lock()
	defer s.contentProtectionMu.Unlock()
	lease, ok := s.contentProtectionLeases[binding.LeaseID]
	if !ok || lease.binding != binding {
		return nil, nil, 0, capability.ErrInvalidCapability
	}
	s.qrRegionMu.Lock()
	defer s.qrRegionMu.Unlock()
	if s.qrRegionCancel != nil {
		return nil, nil, 0, qrregion.ErrBusy
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.qrRegionOperation++
	s.qrRegionCancel = cancel
	s.qrRegionBinding = binding
	return ctx, cancel, s.qrRegionOperation, nil
}

func (s *Service) finishQRRegionSelection(operation uint64) {
	s.qrRegionMu.Lock()
	defer s.qrRegionMu.Unlock()
	if s.qrRegionOperation != operation {
		return
	}
	s.qrRegionCancel = nil
	s.qrRegionBinding = capability.Binding{}
}

func (s *Service) cancelQRRegionSelection(leaseID string) {
	s.qrRegionMu.Lock()
	defer s.qrRegionMu.Unlock()
	if s.qrRegionCancel == nil || (leaseID != "" && s.qrRegionBinding.LeaseID != leaseID) {
		return
	}
	s.qrRegionCancel()
}

// finishQRScan takes the source ("screenshot", "window", "region") for logging
// only; it never records candidate payloads.
func (s *Service) finishQRScan(source, accountID, token string, original qrattempt.Binding, candidates []qrimage.Candidate) (QRScanResult, error) {
	defer clearQRCandidates(candidates)
	// Re-authorized rather than trusted from the start of the scan: a capture can
	// take as long as the user takes to drag a box or pick a file, and the vault
	// may have been locked or the account removed in that time.
	//
	// Only the account is held against the binding the scan opened with. The
	// generation is deliberately not: a background session-token sweep carries
	// live capabilities onto the generation it commits, so the two legitimately
	// differ here, and demanding they match refused scans whose pixels were
	// already captured and decoded. Any write that does not carry capabilities
	// across has already failed the token above.
	current, err := s.authorizeQRBinding(accountID, token)
	if err != nil || current.AccountID != original.AccountID {
		if err != nil {
			logQRFailure(source, original.AccountID, "", err)
			return QRScanResult{}, err
		}
		logQRFailure(source, original.AccountID, "", capability.ErrInvalidCapability)
		return QRScanResult{}, capability.ErrInvalidCapability
	}
	if len(candidates) == 0 {
		logQRFailure(source, original.AccountID, QRScanNoCode, nil)
		return QRScanResult{State: QRScanNoCode}, nil
	}
	if len(candidates) != 1 {
		logQRFailure(source, original.AccountID, QRScanMultipleCodes, nil)
		return QRScanResult{State: QRScanMultipleCodes, CandidateCount: len(candidates)}, nil
	}
	if s.qrAttempts == nil {
		logQRFailure(source, original.AccountID, QRScanUnavailable, nil)
		return QRScanResult{State: QRScanUnavailable}, nil
	}
	payload := []byte(candidates[0].Payload)
	candidates[0].Payload = ""
	attempt, err := s.qrAttempts.Create(current, payload, qrAttemptLifetime)
	if err != nil {
		logQRFailure(source, original.AccountID, QRScanUnavailable, err)
		return QRScanResult{State: QRScanUnavailable}, nil
	}
	qrLogger().Info("Steam QR code decoded", "source", source, "steamId64", original.AccountID)
	return QRScanResult{State: QRScanReady, Attempt: string(attempt), CandidateCount: 1}, nil
}

func (s *Service) authorizeQRBinding(accountID, token string) (qrattempt.Binding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.requireVaultLocked()
	if err != nil {
		return qrattempt.Binding{}, err
	}
	if err := s.authorizeModalLocked(v, accountID, token); err != nil {
		return qrattempt.Binding{}, err
	}
	if v.IsLocked() {
		return qrattempt.Binding{}, vault.ErrLocked
	}
	accountID = strings.TrimSpace(accountID)
	records, err := v.List()
	if err != nil {
		return qrattempt.Binding{}, err
	}
	for _, record := range records {
		if record.SteamID64 == accountID {
			return qrattempt.Binding{AccountID: accountID, VaultGeneration: v.Generation()}, nil
		}
	}
	return qrattempt.Binding{}, ErrAccountNotFound
}

func (s *Service) authorizeQRAccount(accountID, token string) (qrattempt.Binding, mafile.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.requireVaultLocked()
	if err != nil {
		return qrattempt.Binding{}, mafile.Account{}, err
	}
	if err := s.authorizeModalLocked(v, accountID, token); err != nil {
		return qrattempt.Binding{}, mafile.Account{}, err
	}
	if v.IsLocked() {
		return qrattempt.Binding{}, mafile.Account{}, vault.ErrLocked
	}
	accountID = strings.TrimSpace(accountID)
	records, err := v.List()
	if err != nil {
		return qrattempt.Binding{}, mafile.Account{}, err
	}
	for _, record := range records {
		if record.SteamID64 != accountID {
			continue
		}
		account, accountErr := accountFromRecord(v, record.ID)
		if accountErr != nil {
			return qrattempt.Binding{}, mafile.Account{}, accountErr
		}
		return qrattempt.Binding{AccountID: accountID, VaultGeneration: v.Generation()}, account, nil
	}
	return qrattempt.Binding{}, mafile.Account{}, ErrAccountNotFound
}

func mobileAccessToken(account mafile.Account, accountID string) (string, error) {
	if account.Session == nil || strconv.FormatUint(account.Session.SteamID, 10) != accountID {
		return "", ErrSteamMobileSession
	}
	token := strings.TrimSpace(account.Session.AccessToken)
	if len(token) < 16 || len(token) > 4096 {
		return "", ErrSteamMobileSession
	}
	return token, nil
}

func qrApprovalView(accountName string, info protocol.AuthSessionInfo) QRApprovalView {
	location := strings.TrimSpace(strings.Join(nonEmpty(info.City, info.State, info.Country), ", "))
	if info.GeoLocation != "" {
		location = info.GeoLocation
	}
	return QRApprovalView{
		AccountName:              accountName,
		DeviceName:               info.DeviceFriendlyName,
		IPAddress:                info.IPAddress,
		Location:                 location,
		Platform:                 qrPlatformLabel(info.Platform),
		Application:              qrAppLabel(info.App),
		Persistence:              qrPersistenceLabel(info.RequestedPersistence),
		LocationMismatch:         info.RequestorLocationMismatch,
		HighUsageLogin:           info.HighUsageLogin,
		PreviouslyUsedLocation:   info.LoginHistory == protocol.SecurityHistoryUsedPreviously,
		RequestorDeviceTrustCode: info.DeviceTrust,
	}
}

func nonEmpty(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func qrPlatformLabel(value protocol.PlatformType) string {
	switch value {
	case protocol.PlatformSteamClient:
		return "Steam client"
	case protocol.PlatformWebBrowser:
		return "Web browser"
	case protocol.PlatformMobileApp:
		return "Mobile app"
	default:
		return "Unknown"
	}
}

func qrAppLabel(value protocol.AppType) string {
	switch value {
	case protocol.AppTypeSteamMobile:
		return "Steam Mobile"
	case protocol.AppTypeSteamChat:
		return "Steam Chat"
	default:
		return "Steam"
	}
}

func qrPersistenceLabel(value protocol.Persistence) string {
	if value == protocol.PersistencePersistent {
		return "Remembered session"
	}
	return "This session only"
}

func decodeFailureResult(err error) QRScanResult {
	if errors.Is(err, qrimage.ErrTooManyFrames) || errors.Is(err, qrimage.ErrDecodeWorkLimit) ||
		errors.Is(err, qrimage.ErrDecodeTimeout) || errors.Is(err, context.DeadlineExceeded) {
		return QRScanResult{State: QRScanWorkLimit}
	}
	return QRScanResult{State: QRScanInvalidImage}
}

func qrRegionFailureResult(err error) QRScanResult {
	switch {
	case errors.Is(err, qrregion.ErrCanceled), errors.Is(err, context.Canceled):
		return QRScanResult{State: QRScanCanceled}
	case errors.Is(err, qrregion.ErrBusy):
		return QRScanResult{State: QRScanBusy}
	case errors.Is(err, qrregion.ErrUnsupported):
		return QRScanResult{State: QRScanUnsupported}
	default:
		return QRScanResult{State: QRScanCaptureFailed}
	}
}

func clearQRCandidates(candidates []qrimage.Candidate) {
	for i := range candidates {
		candidates[i].Payload = ""
	}
}
