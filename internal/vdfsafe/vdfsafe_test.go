package vdfsafe

import (
	"strings"
	"testing"
)

// A key whose value never arrived - steamvdf indexes past the end of the token
// and panics instead of reporting a parse error.
const truncatedVDF = "\"users\"\n{\n\t\"76561198000000100\"\n\t{\n\t\t\"AccountName\""

func TestReadBytes_truncatedReturnsError(t *testing.T) {
	kv, err := ReadBytes([]byte(truncatedVDF))
	if err == nil {
		t.Fatalf("ReadBytes(truncated) = %+v, nil; want an error", kv)
	}
	if !strings.Contains(err.Error(), "malformed VDF") {
		t.Errorf("err = %q, want it to mention malformed VDF", err)
	}
	if len(kv.Children) != 0 {
		t.Errorf("kv has %d children, want the zero value on failure", len(kv.Children))
	}
}

// The recover must not turn good input into a failure.
func TestReadBytes_wellFormedStillParses(t *testing.T) {
	kv, err := ReadBytes([]byte(truncatedVDF + "\t\t\"kevin\"\n\t}\n}\n"))
	if err != nil {
		t.Fatalf("ReadBytes(valid) error: %v", err)
	}
	if len(kv.Children) == 0 {
		t.Errorf("kv = %+v, want the parsed tree", kv)
	}
}
