//go:build windows

package webview2

import (
	"fmt"
	"os"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestValidateProfileName(t *testing.T) {
	valid := []string{
		"acct-76561198000000001",
		"a",
		"A_b.c-d",
		"0123456789",
	}
	for _, name := range valid {
		if err := ValidateProfileName(name); err != nil {
			t.Errorf("ValidateProfileName(%q) = %v, want nil", name, err)
		}
	}

	invalid := map[string]string{
		"empty":          "",
		"leading dot":    ".hidden",
		"leading dash":   "-flag",
		"path separator": "acct/../escape",
		"backslash":      `acct\escape`,
		"space":          "acct 1",
		"colon":          "acct:1",
		"non-ascii":      "acct-é",
		"too long":       string(make([]byte, 0, 65)) + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	for label, name := range invalid {
		if err := ValidateProfileName(name); err == nil {
			t.Errorf("ValidateProfileName(%q) [%s] = nil, want error", name, label)
		}
	}
}

// comBase supplies the IUnknown methods the completion handlers need. Lifetime is
// owned by Go for the length of the test, so refcounting is a no-op.
type comBase struct{}

func (comBase) QueryInterface(_, _ uintptr) uintptr { return 0 }
func (comBase) AddRef() uintptr                     { return 1 }
func (comBase) Release() uintptr                    { return 1 }

type cookieResult struct {
	comBase
	found []string
	done  bool
}

func (r *cookieResult) GetCookiesCompleted(_ uintptr, list *ICoreWebView2CookieList) uintptr {
	defer func() { r.done = true }()
	if list == nil {
		return 0
	}
	count, err := list.GetCount()
	if err != nil {
		return 0
	}
	for i := uint32(0); i < count; i++ {
		cookie, err := list.GetItem(i)
		if err != nil || cookie == nil {
			continue
		}
		name, nameErr := cookie.GetName()
		value, valueErr := cookie.GetValue()
		if nameErr == nil && valueErr == nil {
			r.found = append(r.found, name+"="+value)
		}
		cookie.Release()
	}
	return 0
}

var (
	user32            = windows.NewLazySystemDLL("user32.dll")
	procRegisterCls   = user32.NewProc("RegisterClassExW")
	procCreateWindow  = user32.NewProc("CreateWindowExW")
	procDefWindowProc = user32.NewProc("DefWindowProcW")
	procPeekMessage   = user32.NewProc("PeekMessageW")
	procTranslateMsg  = user32.NewProc("TranslateMessage")
	procDispatchMsg   = user32.NewProc("DispatchMessageW")
	procDestroyWindow = user32.NewProc("DestroyWindow")

	kernel32            = windows.NewLazySystemDLL("kernel32.dll")
	procGetModuleHandle = kernel32.NewProc("GetModuleHandleW")
)

type wndClassExW struct {
	size, style                        uint32
	wndProc                            uintptr
	clsExtra, wndExtra                 int32
	instance, icon, cursor, background windows.Handle
	menuName, className                *uint16
	iconSm                             windows.Handle
}

type msgW struct {
	hwnd     windows.Handle
	message  uint32
	wParam   uintptr
	lParam   uintptr
	time     uint32
	pt       struct{ x, y int32 }
	lPrivate uint32
}

func scenarioWindow() (uintptr, error) {
	instance, _, _ := procGetModuleHandle.Call(0)
	className := windows.StringToUTF16Ptr("TcNoWebView2ProfileTest")
	cls := wndClassExW{
		size:     uint32(unsafe.Sizeof(wndClassExW{})),
		instance: windows.Handle(instance),
		wndProc: windows.NewCallback(func(hwnd windows.Handle, m uint32, w, l uintptr) uintptr {
			r, _, _ := procDefWindowProc.Call(uintptr(hwnd), uintptr(m), w, l)
			return r
		}),
		className: className,
	}
	procRegisterCls.Call(uintptr(unsafe.Pointer(&cls)))
	const wsOverlappedWindow = 0x00CF0000
	hwnd, _, err := procCreateWindow.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("webview2 test"))),
		wsOverlappedWindow,
		0, 0, 640, 480, 0, 0, instance, 0,
	)
	if hwnd == 0 {
		return 0, fmt.Errorf("CreateWindowExW: %w", err)
	}
	return hwnd, nil
}

// pump runs the message loop, which is how WebView2 delivers its completions.
func pump(done func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	var m msgW
	for !done() {
		if time.Now().After(deadline) {
			return false
		}
		r, _, _ := procPeekMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, 1)
		if r == 0 {
			time.Sleep(time.Millisecond)
			continue
		}
		procTranslateMsg.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMsg.Call(uintptr(unsafe.Pointer(&m)))
	}
	return true
}

// sessionsUnderTest maps a profile to the session value only it should ever see.
var sessionsUnderTest = map[string]string{
	"acct-76561198000000001": "sessionA",
	"acct-76561198000000002": "sessionB",
}

// isolation carries the scenario's outcome from TestMain to the test function.
var isolation struct {
	skip string
	err  error
	seen map[string][]string
}

// TestMain runs the WebView2 scenario on the process main thread, then hands the
// result to the test function.
//
// This shape is forced by COM. Everything here is thread affine to the apartment
// this package's init sets up, and init pins the main goroutine to the main thread
// to establish it — which is also exactly how the application runs, on the Wails
// main thread. The testing framework runs test functions on a different goroutine,
// where the same calls fail with RPC_E_WRONG_THREAD, so the scenario cannot live
// inside the test function itself.
func TestMain(m *testing.M) {
	runIsolationScenario()
	os.Exit(m.Run())
}

func runIsolationScenario() {
	dir, err := os.MkdirTemp("", "webview2-profile-test")
	if err != nil {
		isolation.err = err
		return
	}
	defer os.RemoveAll(dir)

	const domain = "steamcommunity.com"
	const uri = "https://steamcommunity.com/"

	// One Chromium per profile, all sharing a user data folder, which is how the
	// application opens one browser window per account.
	jars := make(map[string]*ICoreWebView2CookieManager, len(sessionsUnderTest))
	for profile, session := range sessionsUnderTest {
		hwnd, err := scenarioWindow()
		if err != nil {
			isolation.err = err
			return
		}
		defer procDestroyWindow.Call(hwnd)

		c := NewChromium()
		c.DataPath = dir
		c.ProfileName = profile
		if !c.Embed(hwnd) {
			isolation.skip = "WebView2 runtime unavailable"
			return
		}
		if !c.Environment().SupportsProfiles() {
			isolation.skip = "WebView2 runtime predates profile support (110)"
			return
		}

		jar, err := c.GetCookieManager()
		if err != nil {
			isolation.err = fmt.Errorf("cookie manager for %q: %w", profile, err)
			return
		}
		cookie, err := jar.CreateCookie("steamLoginSecure", session, domain, "/")
		if err != nil {
			isolation.err = fmt.Errorf("create cookie for %q: %w", profile, err)
			return
		}
		if err := jar.AddOrUpdateCookie(cookie); err != nil {
			isolation.err = fmt.Errorf("write cookie for %q: %w", profile, err)
			return
		}
		jars[profile] = jar
	}

	isolation.seen = make(map[string][]string, len(jars))
	for profile, jar := range jars {
		read := &cookieResult{}
		if err := jar.GetCookies(uri, NewICoreWebView2GetCookiesCompletedHandler(read)); err != nil {
			isolation.err = fmt.Errorf("read cookies for %q: %w", profile, err)
			return
		}
		if !pump(func() bool { return read.done }, 30*time.Second) {
			isolation.err = fmt.Errorf("timed out reading cookies for %q", profile)
			return
		}
		isolation.seen[profile] = read.found
	}
}

// TestProfilesIsolateCookies guards the hand-derived vtable layouts in profile.go
// and the corrected asynchronous GetCookies. A wrong slot offset does not fail
// loudly — it calls a different method and returns a plausible HRESULT — so the
// only way to know these are right is to drive the real runtime and see that two
// profiles genuinely cannot see each other's session.
func TestProfilesIsolateCookies(t *testing.T) {
	if isolation.skip != "" {
		t.Skip(isolation.skip)
	}
	if isolation.err != nil {
		t.Fatal(isolation.err)
	}
	for profile, session := range sessionsUnderTest {
		want := "steamLoginSecure=" + session
		got := isolation.seen[profile]
		if len(got) != 1 || got[0] != want {
			t.Errorf("profile %q sees %v, want exactly [%s] — the profiles are sharing a cookie jar",
				profile, got, want)
		}
	}
}
