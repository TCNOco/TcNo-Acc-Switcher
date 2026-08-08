package steam

import (
	"fmt"

	"github.com/Jleagle/steam-go/steamvdf"
)

// readVDFBytes parses text or binary VDF, turning a malformed file into an
// error instead of a crash.
//
// steamvdf panics on truncated input rather than returning an error, and every
// VDF this package reads is a file some other program owns - Steam, a cleaning
// tool or a crash mid-write can all leave one half-finished. Callers need to
// tell "unreadable" from "absent" to decide whether overwriting it is safe, and
// a panic answers neither question.
func readVDFBytes(raw []byte) (kv steamvdf.KeyValue, err error) {
	defer func() {
		if r := recover(); r != nil {
			kv = steamvdf.KeyValue{}
			err = fmt.Errorf("malformed VDF: %v", r)
		}
	}()
	return steamvdf.ReadBytes(raw)
}
