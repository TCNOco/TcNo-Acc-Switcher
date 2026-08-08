// Package vdfsafe wraps Valve KeyValues parsing so a malformed file cannot take
// the process down.
//
// It is a leaf package on purpose: internal/steam already depends on
// internal/logsanitize through crashlog, so the shared guard cannot live in
// either of them.
package vdfsafe

import (
	"fmt"

	"github.com/Jleagle/steam-go/steamvdf"
)

// ReadBytes parses text or binary VDF, turning a malformed file into an error
// instead of a crash.
//
// steamvdf panics on truncated input rather than returning an error, and every
// VDF we read is a file some other program owns - Steam, a cleaning tool or a
// crash mid-write can all leave one half-finished. Callers need to tell
// "unreadable" from "absent" to decide whether overwriting it is safe, and a
// panic answers neither question.
func ReadBytes(raw []byte) (kv steamvdf.KeyValue, err error) {
	defer func() {
		if r := recover(); r != nil {
			kv = steamvdf.KeyValue{}
			err = fmt.Errorf("malformed VDF: %v", r)
		}
	}()
	return steamvdf.ReadBytes(raw)
}
