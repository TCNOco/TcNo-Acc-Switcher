//go:build !windows

package winutil

// TrayIconSize reports no preferred size off Windows.
//
// A StatusNotifierItem host is handed the pixmap as-is and scales it to the
// panel itself, so the largest source available is the right one to send - the
// opposite of what Windows wants.
func TrayIconSize() int { return 0 }
