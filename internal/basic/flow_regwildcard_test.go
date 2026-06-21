package basic

import (
	"path/filepath"
	"testing"
)

func TestRegDumpLookupWildcardBundle(t *testing.T) {
	m := map[string]regDumpEntry{
		`REG:HKCU\Software\TestApp:*`: {
			Values: map[string]regDumpEntry{
				"Token": {V: "abc", T: 1},
			},
		},
	}
	e, ok := regDumpLookup(m, `REG:HKCU\Software\TestApp:Token`)
	if !ok || e.V != "abc" || e.T != 1 {
		t.Fatalf("lookup: ok=%v entry=%+v", ok, e)
	}
}

func TestRegDumpLookupValueNameGlobBundle(t *testing.T) {
	m := map[string]regDumpEntry{
		`REG:HKCU\Software\TestApp:LastLoginDate_*`: {
			Values: map[string]regDumpEntry{
				"LastLoginDate_h202577560": {V: "d1", T: 1},
				"LastLoginDate_1234":       {V: "d2", T: 1},
			},
		},
	}
	e, ok := regDumpLookup(m, `REG:HKCU\Software\TestApp:LastLoginDate_h202577560`)
	if !ok || e.V != "d1" {
		t.Fatalf("lookup: ok=%v entry=%+v", ok, e)
	}
	if ok, _ := filepath.Match(`LastLoginDate_*`, "LastLoginDate_h202577560"); !ok {
		t.Fatal("filepath.Match sanity")
	}
}

func TestFirstValueNameMatchingGlobStableOrder(t *testing.T) {
	v := map[string]regDumpEntry{
		"LastLoginString_z": {V: "z", T: 1},
		"LastLoginString_a": {V: "a", T: 1},
	}
	got := firstValueNameMatchingGlob(v, "LastLoginString_*")
	if got != "LastLoginString_a" {
		t.Fatalf("got %q want lexicographically smallest matching name", got)
	}
}
