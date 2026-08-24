package winutil

// smCXSMICON is SM_CXSMICON, the small-icon metric the notification area uses.
const smCXSMICON = 49

var procGetSystemMetricsTray = modUser32.NewProc("GetSystemMetrics")

// TrayIconSize is the edge length Windows wants a notification-area icon to be
// at the current DPI: 16 at 100%, 24 at 150%, 32 at 200%.
//
// Wails converts whatever PNG it is handed with CreateIconFromResourceEx at
// exactly this size, so a source of any other size is rescaled by GDI. Feeding
// it a bitmap already at this size makes that a straight copy instead.
func TrayIconSize() int {
	v, _, _ := procGetSystemMetricsTray.Call(uintptr(smCXSMICON))
	return int(int32(v))
}
