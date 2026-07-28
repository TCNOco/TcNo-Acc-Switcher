//go:build windows

package app

import "testing"

func TestWindowsSecurityTrigger(t *testing.T) {
	tests := []struct {
		name    string
		msg     uint32
		wParam  uintptr
		trigger securityLifecycleTrigger
		ok      bool
	}{
		{name: "query end session", msg: wmQueryEndSession, trigger: securityTriggerSessionEnd, ok: true},
		{name: "confirmed end session", msg: wmEndSession, wParam: 1, trigger: securityTriggerSessionEnd, ok: true},
		{name: "cancelled end session", msg: wmEndSession},
		{name: "workstation lock", msg: wmWTSSessionChange, wParam: wtsSessionLock, trigger: securityTriggerScreenLock, ok: true},
		{name: "session logoff", msg: wmWTSSessionChange, wParam: wtsSessionLogoff, trigger: securityTriggerScreenLock, ok: true},
		{name: "session unlock", msg: wmWTSSessionChange, wParam: 0x8},
		{name: "unrelated", msg: 0x1234},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			trigger, ok := windowsSecurityTrigger(test.msg, test.wParam)
			if ok != test.ok || trigger != test.trigger {
				t.Fatalf("got (%q, %v), want (%q, %v)", trigger, ok, test.trigger, test.ok)
			}
		})
	}
}
