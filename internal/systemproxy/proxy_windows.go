package systemproxy

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	winhttp        = windows.NewLazySystemDLL("winhttp.dll")
	getUserConfig  = winhttp.NewProc("WinHttpGetIEProxyConfigForCurrentUser")
	openSession    = winhttp.NewProc("WinHttpOpen")
	closeSession   = winhttp.NewProc("WinHttpCloseHandle")
	setTimeouts    = winhttp.NewProc("WinHttpSetTimeouts")
	getProxyForURL = winhttp.NewProc("WinHttpGetProxyForUrl")
	globalFree     = windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalFree")
)

type userConfig struct {
	AutoDetect    int32
	AutoConfigURL *uint16
	Proxy         *uint16
	Bypass        *uint16
}

type autoProxyOptions struct {
	Flags           uint32
	AutoDetectFlags uint32
	AutoConfigURL   *uint16
	Reserved        unsafe.Pointer
	ReservedFlags   uint32
	AutoLogon       int32
}

type proxyInfo struct {
	AccessType uint32
	Proxy      *uint16
	Bypass     *uint16
}

func freeString(p *uint16) {
	if p != nil {
		globalFree.Call(uintptr(unsafe.Pointer(p)))
	}
}

// Proxy reads the current user's Windows Internet Options on each request, so
// changing a VPN's system proxy takes effect without restarting the app.
// Windows settings are authoritative here; process proxy variables cannot
// override them. A proxy connection failure is never retried directly.
func Proxy(req *http.Request) (*url.URL, error) {
	if !validRequest(req) {
		return nil, errInvalidProxy
	}
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	var config userConfig
	ok, _, callErr := getUserConfig.Call(uintptr(unsafe.Pointer(&config)))
	if ok == 0 {
		return nil, fmt.Errorf("read Windows proxy settings: %w", callErr)
	}
	defer freeString(config.AutoConfigURL)
	defer freeString(config.Proxy)
	defer freeString(config.Bypass)
	return proxyForWindowsConfig(req, config, automaticProxy)
}

func proxyForWindowsConfig(req *http.Request, config userConfig, resolve func(*http.Request, userConfig) (*url.URL, error)) (*url.URL, error) {
	server := windows.UTF16PtrToString(config.Proxy)
	bypass := windows.UTF16PtrToString(config.Bypass)
	if config.AutoConfigURL != nil || config.AutoDetect != 0 {
		proxy, err := resolve(req, config)
		if err == nil {
			return proxy, nil
		}
		// Windows commonly enables discovery on networks without WPAD. Only a
		// confirmed absence of discovery may fall back to manual settings/direct.
		if config.AutoConfigURL != nil || !errors.Is(err, windows.Errno(12180)) {
			return nil, err
		}
	}
	return proxyForConfig(req.URL, server, bypass)
}

func automaticProxy(req *http.Request, config userConfig) (*url.URL, error) {
	agent, _ := windows.UTF16PtrFromString("TcNo Account Switcher")
	session, _, err := openSession.Call(uintptr(unsafe.Pointer(agent)), 1, 0, 0, 0)
	if session == 0 {
		return nil, fmt.Errorf("open Windows proxy resolver: %w", err)
	}
	defer closeSession.Call(session)
	// Bound native network operations; check the request deadline before/after
	// the synchronous WinHTTP resolver as it does not accept a Go context.
	ok, _, err := setTimeouts.Call(session, 3000, 3000, 3000, 3000)
	if ok == 0 {
		return nil, fmt.Errorf("set Windows proxy timeouts: %w", err)
	}
	options := autoProxyOptions{}
	if config.AutoConfigURL != nil {
		options.Flags = 2 // WINHTTP_AUTOPROXY_CONFIG_URL
		options.AutoConfigURL = config.AutoConfigURL
	} else {
		options.Flags = 1           // WINHTTP_AUTOPROXY_AUTO_DETECT
		options.AutoDetectFlags = 3 // DHCP and DNS
	}
	target, err := windows.UTF16PtrFromString(req.URL.String())
	if err != nil {
		return nil, errInvalidProxy
	}
	var info proxyInfo
	ok, _, err = getProxyForURL.Call(session, uintptr(unsafe.Pointer(target)), uintptr(unsafe.Pointer(&options)), uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return nil, fmt.Errorf("resolve Windows automatic proxy: %w", err)
	}
	defer freeString(info.Proxy)
	defer freeString(info.Bypass)
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	if info.AccessType == 1 {
		return nil, nil
	} // WINHTTP_ACCESS_TYPE_NO_PROXY
	if info.AccessType != 3 || info.Proxy == nil {
		return nil, errInvalidProxy
	}
	return proxyForConfig(req.URL, windows.UTF16PtrToString(info.Proxy), windows.UTF16PtrToString(info.Bypass))
}
