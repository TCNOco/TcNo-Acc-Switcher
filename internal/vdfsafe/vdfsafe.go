// Package vdfsafe wraps Valve KeyValues parsing so a malformed file cannot take
// the process down, and so text values arrive unescaped.
//
// It is a leaf package on purpose: internal/steam already depends on
// internal/logsanitize through crashlog, so the shared guard cannot live in
// either of them.
package vdfsafe

import (
	"fmt"
	"strings"

	"github.com/Jleagle/steam-go/steamvdf"
)

// escapes drives both directions: [Escape] and [Unescape] must stay exact inverses.
var escapes = []struct {
	plain   string
	escaped string
}{
	{plain: `\`, escaped: `\\`},
	{plain: `"`, escaped: `\"`},
	{plain: "\n", escaped: `\n`},
	{plain: "\r", escaped: `\r`},
	{plain: "\t", escaped: `\t`},
}

// ReadBytes parses text or binary VDF, turning a malformed file into an error
// instead of a crash, and returns text values with their escapes resolved.
//
// steamvdf panics on truncated input rather than returning an error, and every
// VDF we read is a file some other program owns - Steam, a cleaning tool or a
// crash mid-write can all leave one half-finished. Callers need to tell
// "unreadable" from "absent" to decide whether overwriting it is safe, and a
// panic answers neither question.
//
// steamvdf's text parser hands back the raw bytes between the quotes, still
// escaped, so writing a parsed tree back doubles every backslash. Resolving
// escapes here is what makes [Escape] a true inverse.
//
// Binary VDF carries no escapes; the same pass would eat backslashes that belong
// to the data.
func ReadBytes(raw []byte) (kv steamvdf.KeyValue, err error) {
	defer func() {
		if r := recover(); r != nil {
			kv = steamvdf.KeyValue{}
			err = fmt.Errorf("malformed VDF: %v", r)
		}
	}()
	kv, err = steamvdf.ReadBytes(raw)
	if err != nil {
		return steamvdf.KeyValue{}, err
	}
	if !steamvdf.IsBinary(raw) {
		unescapeTree(&kv)
	}
	return kv, nil
}

// Escape renders a value for text VDF, inverting what [ReadBytes] resolves.
func Escape(s string) string {
	for _, e := range escapes {
		s = strings.ReplaceAll(s, e.plain, e.escaped)
	}
	return s
}

func unescapeTree(kv *steamvdf.KeyValue) {
	kv.Key = Unescape(kv.Key)
	kv.Value = Unescape(kv.Value)
	for i := range kv.Children {
		unescapeTree(&kv.Children[i])
	}
}

// Unescape resolves the escapes in one text VDF value. A sequence Valve does not
// define is kept exactly as found, backslash included: dropping it would rewrite
// a value we are only passing through, and [Escape] puts the pair back unchanged.
func Unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		matched := false
		for _, e := range escapes {
			if s[i+1] == e.escaped[1] {
				b.WriteString(e.plain)
				i++
				matched = true
				break
			}
		}
		if !matched {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
