package steamguard

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/capability"
	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/protocol"
	"TcNo-Acc-Switcher/internal/steamguard/qr"
	"TcNo-Acc-Switcher/internal/steamguard/qrattempt"
	"TcNo-Acc-Switcher/internal/steamguard/qrcapture"
	"TcNo-Acc-Switcher/internal/steamguard/qrregion"
	"TcNo-Acc-Switcher/internal/steamguard/vault"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/makiuchi-d/gozxing/qrcode/decoder"
)

const (
	qrTestAccountID = "76561198000000000"
	qrTestPassword  = "correct horse battery staple"
	qrTestChallenge = "https://s.team/q/1/1234567890123456789"
)

func TestDecodeQRScreenshotCreatesOpaqueSingleUseAttempt(t *testing.T) {
	service, grant := setupQRService(t)
	path := writeQRPNG(t, qrTestChallenge)

	result, err := service.DecodeQRScreenshot(qrTestAccountID, path, grant.Capability)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != QRScanReady || result.Attempt == "" || result.CandidateCount != 1 {
		t.Fatalf("scan result = %#v", result)
	}
	if bytes.Contains([]byte(result.Attempt), []byte("s.team")) {
		t.Fatal("scan result exposed the challenge")
	}

	binding := qrattempt.Binding{AccountID: qrTestAccountID, VaultGeneration: service.vault.Generation()}
	var challenge string
	if err := service.qrAttempts.Consume(qrattempt.ID(result.Attempt), binding, func(payload []byte) error {
		challenge = string(payload)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if challenge != qrTestChallenge {
		t.Fatalf("challenge = %q", challenge)
	}
	if err := service.qrAttempts.Consume(qrattempt.ID(result.Attempt), binding, func([]byte) error { return nil }); !errors.Is(err, qrattempt.ErrNotFound) {
		t.Fatalf("replayed attempt error = %v", err)
	}
}

func TestDecodeQRScreenshotRejectsLockedVaultAndRevokesStaleAttempt(t *testing.T) {
	service, grant := setupQRService(t)
	ready, err := service.DecodeQRScreenshot(qrTestAccountID, writeQRPNG(t, qrTestChallenge), grant.Capability)
	if err != nil || ready.State != QRScanReady {
		t.Fatalf("initial scan = %#v, %v", ready, err)
	}
	binding := qrattempt.Binding{AccountID: qrTestAccountID, VaultGeneration: service.vault.Generation()}

	invalid, err := service.DecodeQRScreenshot(qrTestAccountID, filepath.Join(tempDir(t), "missing.png"), grant.Capability)
	if err != nil || invalid.State != QRScanInvalidImage {
		t.Fatalf("invalid scan = %#v, %v", invalid, err)
	}
	if err := service.qrAttempts.Consume(qrattempt.ID(ready.Attempt), binding, func([]byte) error { return nil }); !errors.Is(err, qrattempt.ErrNotFound) {
		t.Fatalf("stale attempt error = %v", err)
	}

	if err := service.vault.Lock(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.DecodeQRScreenshot(qrTestAccountID, writeQRPNG(t, qrTestChallenge), grant.Capability); !errors.Is(err, vault.ErrLocked) {
		t.Fatalf("locked scan error = %v", err)
	}
}

func TestScanSteamQRUsesConfiguredPathAndWipesCapture(t *testing.T) {
	service, grant := setupQRService(t)
	frame, backing := makeQRCaptureFrame(t, qrTestChallenge)
	scanner := &fakeSteamQRScanner{
		discovery: qrcapture.Discovery{State: qrcapture.DiscoveryReady, Windows: []qrcapture.Candidate{{Handle: 42}}},
		capture:   qrcapture.Capture{State: qrcapture.CaptureReady, Frames: []qrcapture.Frame{frame}},
	}
	service.qrScanner = scanner
	service.resolveSteamExecutableFn = func() (string, bool) { return `C:\Steam\steam.exe`, true }

	result, err := service.ScanSteamQR(qrTestAccountID, grant.Capability)
	if err != nil || result.State != QRScanReady || result.Attempt == "" {
		t.Fatalf("automatic scan = %#v, %v", result, err)
	}
	if scanner.configuredPath != `C:\Steam\steam.exe` {
		t.Fatalf("configured path = %q", scanner.configuredPath)
	}
	for _, value := range backing {
		if value != 0 {
			t.Fatal("captured pixels were not wiped")
		}
	}
}

func TestSelectQRRegionDecodesAndWipesCapturedPixels(t *testing.T) {
	service, grant := setupQRService(t)
	frame, backing := makeQRRegionFrame(t, qrTestChallenge)
	service.qrRegionSelector = &fakeQRRegionSelector{frame: frame}

	result, err := service.SelectQRRegion(qrTestAccountID, grant.Capability)
	if err != nil || result.State != QRScanReady || result.Attempt == "" {
		t.Fatalf("region scan = %#v, %v", result, err)
	}
	for _, value := range backing {
		if value != 0 {
			t.Fatal("region pixels were not wiped")
		}
	}
}

func TestSelectQRRegionIsBusyAndCapabilityRevocationCancelsActiveSelection(t *testing.T) {
	service, grant := setupQRService(t)
	selector := &fakeQRRegionSelector{started: make(chan struct{}), waitForCancel: true}
	service.qrRegionSelector = selector

	resultCh := make(chan QRScanResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := service.SelectQRRegion(qrTestAccountID, grant.Capability)
		resultCh <- result
		errCh <- err
	}()
	select {
	case <-selector.started:
	case <-time.After(time.Second):
		t.Fatal("region selector did not start")
	}

	busy, err := service.SelectQRRegion(qrTestAccountID, grant.Capability)
	if err != nil || busy.State != QRScanBusy {
		t.Fatalf("concurrent region scan = %#v, %v", busy, err)
	}
	if err := service.EndSensitiveView(grant.Capability, grant.Lease); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-resultCh:
		if err := <-errCh; err != nil || result.State != QRScanCanceled {
			t.Fatalf("canceled region scan = %#v, %v", result, err)
		}
	case <-time.After(time.Second):
		t.Fatal("region selection was not canceled")
	}
}

// A region drag lasts as long as the user takes over it, so a background
// session-token sweep lands in the middle of one sooner or later. The scan must
// not be refused after the pixels have already been captured and decoded.
func TestQRFlowSurvivesABackgroundSessionRefreshMidSelection(t *testing.T) {
	service, grant := setupQRService(t)
	frame, _ := makeQRRegionFrame(t, qrTestChallenge)
	selector := &fakeQRRegionSelector{frame: frame, started: make(chan struct{}), release: make(chan struct{})}
	service.qrRegionSelector = selector
	service.qrAuth = &fakeSteamQRAuthenticator{}

	before := service.vault.Generation()
	resultCh := make(chan QRScanResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := service.SelectQRRegion(qrTestAccountID, grant.Capability)
		resultCh <- result
		errCh <- err
	}()
	select {
	case <-selector.started:
	case <-time.After(time.Second):
		t.Fatal("region selector did not start")
	}

	renewSessionTokensLikeASweep(t, service)
	if service.vault.Generation() == before {
		t.Fatal("the simulated sweep did not rotate the vault generation")
	}
	close(selector.release)

	var result QRScanResult
	select {
	case result = <-resultCh:
		if err := <-errCh; err != nil {
			t.Fatalf("region scan across a sweep: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("region selection did not finish")
	}
	if result.State != QRScanReady || result.Attempt == "" {
		t.Fatalf("region scan across a sweep = %#v", result)
	}

	// The capability the modal still holds has to keep working, and so does the
	// attempt it just produced: the user reads the approval panel and presses
	// Approve well after the sweep has committed.
	if _, err := service.GetQRApproval(qrTestAccountID, result.Attempt, grant.Capability); err != nil {
		t.Fatalf("approval after a sweep: %v", err)
	}
	if err := service.AuthorizeQRLogin(qrTestAccountID, result.Attempt, grant.Capability); err != nil {
		t.Fatalf("authorize after a sweep: %v", err)
	}
}

// A vault write that re-keys or changes which records exist must still orphan
// the capabilities bound to the generation it replaced. Only the session-token
// sweeps get to carry them across.
func TestAnOrdinaryVaultWriteStillInvalidatesTheCapability(t *testing.T) {
	service, grant := setupQRService(t)
	renewSessionTokensWithoutCarrying(t, service)

	_, err := service.ScanSteamQR(qrTestAccountID, grant.Capability)
	if !errors.Is(err, capability.ErrInvalidCapability) {
		t.Fatalf("scan after an uncarried write = %v, want ErrInvalidCapability", err)
	}
}

// renewSessionTokensLikeASweep rewrites the account exactly as RefreshBatch does
// - one PutRecords, one generation - and then carries the open capabilities onto
// it, which is the sweep's contract.
func renewSessionTokensLikeASweep(t *testing.T, service *Service) {
	t.Helper()
	renewSessionTokensWithoutCarrying(t, service)
	service.carryCapabilitiesAcross(service.vault.Generation())
}

func renewSessionTokensWithoutCarrying(t *testing.T, service *Service) {
	t.Helper()
	records, err := service.vault.List()
	if err != nil {
		t.Fatal(err)
	}
	updates := make([]vault.RecordUpdate, 0, len(records))
	for _, record := range records {
		account, err := accountFromRecord(service.vault, record.ID)
		if err != nil {
			t.Fatal(err)
		}
		account.Session.AccessToken = "mobile-access-token-0002"
		plaintext, err := mafile.ExportPlaintext(account, mafile.ExportOptions{IncludeTokens: true})
		if err != nil {
			t.Fatal(err)
		}
		updates = append(updates, vault.RecordUpdate{SteamID64: record.SteamID64, Plaintext: plaintext})
	}
	if err := service.vault.PutRecords(updates); err != nil {
		t.Fatal(err)
	}
}

func TestSelectQRRegionMapsSafePlatformFailures(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		state QRScanState
	}{
		{name: "canceled", err: qrregion.ErrCanceled, state: QRScanCanceled},
		{name: "busy", err: qrregion.ErrBusy, state: QRScanBusy},
		{name: "unsupported", err: qrregion.ErrUnsupported, state: QRScanUnsupported},
		{name: "capture", err: qrregion.ErrCapture, state: QRScanCaptureFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, grant := setupQRService(t)
			service.qrRegionSelector = &fakeQRRegionSelector{err: test.err}
			result, err := service.SelectQRRegion(qrTestAccountID, grant.Capability)
			if err != nil || result.State != test.state {
				t.Fatalf("region failure = %#v, %v", result, err)
			}
		})
	}
}

func TestQRApprovalShowsRequestorThenConsumesAttempt(t *testing.T) {
	service, grant := setupQRService(t)
	ready, err := service.DecodeQRScreenshot(qrTestAccountID, writeQRPNG(t, qrTestChallenge), grant.Capability)
	if err != nil || ready.State != QRScanReady {
		t.Fatalf("scan = %#v, %v", ready, err)
	}
	fake := &fakeSteamQRAuthenticator{info: protocol.AuthSessionInfo{
		IPAddress: "203.0.113.7", City: "Cape Town", Country: "ZA",
		DeviceFriendlyName: "Steam on Windows", Platform: protocol.PlatformSteamClient,
		App: protocol.AppTypeSteamMobile, RequestedPersistence: protocol.PersistencePersistent,
		LoginHistory: protocol.SecurityHistoryUsedPreviously,
	}}
	service.qrAuth = fake

	view, err := service.GetQRApproval(qrTestAccountID, ready.Attempt, grant.Capability)
	if err != nil {
		t.Fatal(err)
	}
	if view.AccountName != "qr_account" || view.DeviceName != "Steam on Windows" || view.Location != "Cape Town, ZA" || view.Persistence != "Remembered session" {
		t.Fatalf("approval view = %#v", view)
	}
	if len(fake.infoRequests) != 1 || fake.infoRequests[0].ClientID != 1234567890123456789 || fake.infoRequests[0].AccessToken != "mobile-access-token-0001" {
		t.Fatalf("info requests = %#v", fake.infoRequests)
	}

	if err := service.AuthorizeQRLogin(qrTestAccountID, ready.Attempt, grant.Capability); err != nil {
		t.Fatal(err)
	}
	if len(fake.confirmRequests) != 1 {
		t.Fatalf("confirmation requests = %#v", fake.confirmRequests)
	}
	request := fake.confirmRequests[0]
	secret, _ := base64.StdEncoding.DecodeString(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 20)))
	expected, err := qr.SignChallenge(qr.Challenge{Version: 1, ClientID: 1234567890123456789}, 76561198000000000, secret)
	if err != nil {
		t.Fatal(err)
	}
	if !request.Confirm || request.Persistence != protocol.PersistencePersistent || request.SteamID != 76561198000000000 || !bytes.Equal(request.Signature, expected) {
		t.Fatalf("confirmation request = %#v", request)
	}
	binding := qrattempt.Binding{AccountID: qrTestAccountID, VaultGeneration: service.vault.Generation()}
	if err := service.qrAttempts.Inspect(qrattempt.ID(ready.Attempt), binding, func([]byte) error { return nil }); !errors.Is(err, qrattempt.ErrNotFound) {
		t.Fatalf("consumed attempt error = %v", err)
	}
}

func TestDismissQRLoginRevokesMatchingAttempt(t *testing.T) {
	service, grant := setupQRService(t)
	ready, err := service.DecodeQRScreenshot(qrTestAccountID, writeQRPNG(t, qrTestChallenge), grant.Capability)
	if err != nil || ready.State != QRScanReady {
		t.Fatalf("scan = %#v, %v", ready, err)
	}
	if err := service.DismissQRLogin(qrTestAccountID, ready.Attempt, grant.Capability); err != nil {
		t.Fatal(err)
	}
	binding := qrattempt.Binding{AccountID: qrTestAccountID, VaultGeneration: service.vault.Generation()}
	if err := service.qrAttempts.Inspect(qrattempt.ID(ready.Attempt), binding, func([]byte) error { return nil }); !errors.Is(err, qrattempt.ErrNotFound) {
		t.Fatalf("dismissed attempt error = %v", err)
	}
}

type fakeSteamQRScanner struct {
	discovery      qrcapture.Discovery
	capture        qrcapture.Capture
	configuredPath string
}

type fakeQRRegionSelector struct {
	frame         qrregion.Frame
	err           error
	started       chan struct{}
	waitForCancel bool
	// release holds the selection open the way a real user does, then completes
	// it normally - what waitForCancel cannot do, since it only ever cancels.
	release chan struct{}
}

func (f *fakeQRRegionSelector) Select(ctx context.Context) (qrregion.Frame, error) {
	if f.started != nil {
		close(f.started)
	}
	if f.release != nil {
		select {
		case <-f.release:
		case <-ctx.Done():
			return qrregion.Frame{}, ctx.Err()
		}
		return f.frame, f.err
	}
	if f.waitForCancel {
		<-ctx.Done()
		return qrregion.Frame{}, ctx.Err()
	}
	return f.frame, f.err
}

type fakeSteamQRAuthenticator struct {
	info            protocol.AuthSessionInfo
	infoErr         error
	confirmErr      error
	infoRequests    []protocol.AuthSessionInfoRequest
	confirmRequests []protocol.MobileConfirmationRequest
}

func (f *fakeSteamQRAuthenticator) GetAuthSessionInfo(_ context.Context, request protocol.AuthSessionInfoRequest, _ time.Duration) (protocol.AuthSessionInfo, error) {
	f.infoRequests = append(f.infoRequests, request)
	return f.info, f.infoErr
}

func (f *fakeSteamQRAuthenticator) UpdateAuthSessionWithMobileConfirmation(_ context.Context, request protocol.MobileConfirmationRequest, _ time.Duration) (protocol.ChallengeResult, error) {
	request.Signature = append([]byte(nil), request.Signature...)
	f.confirmRequests = append(f.confirmRequests, request)
	if f.confirmErr != nil {
		return protocol.ChallengeResult{}, f.confirmErr
	}
	return protocol.ChallengeResult{State: protocol.AuthResultChallengeAccepted}, nil
}

func (f *fakeSteamQRScanner) Discover(configuredPath string) (qrcapture.Discovery, error) {
	f.configuredPath = configuredPath
	return f.discovery, nil
}

func (f *fakeSteamQRScanner) CaptureWindow(qrcapture.Candidate) (qrcapture.Capture, error) {
	return f.capture, nil
}

func setupQRService(t *testing.T) (*Service, SensitiveViewGrant) {
	t.Helper()
	useSettingsRoot(t)
	service := newServiceForTest()
	t.Cleanup(func() {
		if err := service.ServiceShutdown(); err != nil {
			t.Errorf("service shutdown: %v", err)
		}
	})
	service.setMainContentProtectionFn = func(bool) error { return nil }
	if _, err := service.Initialize(qrTestPassword, ""); err != nil {
		t.Fatal(err)
	}
	account := mafile.Account{
		SharedSecret:   base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 20)),
		IdentitySecret: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x24}, 20)),
		DeviceID:       "android:01234567-89ab-cdef-0123-456789abcdef",
		AccountName:    "qr_account",
		FullyEnrolled:  true,
		Session: &mafile.SessionData{
			SteamID: 76561198000000000, AccessToken: "mobile-access-token-0001",
			SessionID: "0123456789ABCDEF0123456789ABCDEF",
		},
	}
	plaintext, err := mafile.ExportPlaintext(account, mafile.ExportOptions{IncludeTokens: true})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(tempDir(t), qrTestAccountID+".maFile")
	if err := os.WriteFile(path, plaintext, 0o600); err != nil {
		t.Fatal(err)
	}
	if results, err := service.ImportPlaintext([]string{path}, qrTestPassword, false); err != nil || len(results) != 1 || !results[0].Imported {
		t.Fatalf("import = %#v, %v", results, err)
	}
	return service, issueSensitiveGrant(t, service, qrTestAccountID, "request-qr-service-0001")
}

func qrTestImage(t *testing.T, payload string) image.Image {
	t.Helper()
	const (
		module    = 6
		quietZone = 4
	)
	matrix, err := qrcode.NewQRCodeWriter().Encode(payload, gozxing.BarcodeFormat_QR_CODE, 0, 0,
		map[gozxing.EncodeHintType]interface{}{
			gozxing.EncodeHintType_ERROR_CORRECTION: decoder.ErrorCorrectionLevel_M,
			gozxing.EncodeHintType_MARGIN:           quietZone,
		})
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, matrix.GetWidth()*module, matrix.GetHeight()*module))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}), image.Point{}, draw.Src)
	for y := 0; y < matrix.GetHeight(); y++ {
		for x := 0; x < matrix.GetWidth(); x++ {
			if !matrix.Get(x, y) {
				continue
			}
			draw.Draw(img, image.Rect(x*module, y*module, (x+1)*module, (y+1)*module),
				image.NewUniform(color.NRGBA{A: 0xff}), image.Point{}, draw.Src)
		}
	}
	return img
}

func writeQRPNG(t *testing.T, payload string) string {
	t.Helper()
	source := qrTestImage(t, payload)
	path := filepath.Join(tempDir(t), "steam-login.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, source); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func makeQRCaptureFrame(t *testing.T, payload string) (qrcapture.Frame, []byte) {
	t.Helper()
	source := qrTestImage(t, payload)
	bounds := source.Bounds()
	stride := bounds.Dx() * 4
	pixels := make([]byte, stride*bounds.Dy())
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			pixel := color.NRGBAModel.Convert(source.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			offset := y*stride + x*4
			pixels[offset] = pixel.B
			pixels[offset+1] = pixel.G
			pixels[offset+2] = pixel.R
			pixels[offset+3] = pixel.A
		}
	}
	return qrcapture.Frame{Width: bounds.Dx(), Height: bounds.Dy(), Stride: stride, BGRA: pixels}, pixels
}

func makeQRRegionFrame(t *testing.T, payload string) (qrregion.Frame, []byte) {
	t.Helper()
	frame, backing := makeQRCaptureFrame(t, payload)
	return qrregion.Frame{
		Region: qrregion.Rect{Right: int32(frame.Width), Bottom: int32(frame.Height)},
		Width:  frame.Width, Height: frame.Height, Stride: frame.Stride, BGRA: frame.BGRA,
	}, backing
}
