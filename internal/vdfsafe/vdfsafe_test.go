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

// The property the package rests on. When it did not hold, every value holding
// a backslash grew one more on each write.
func TestEscapeIsInverseOfUnescape(t *testing.T) {
	t.Parallel()
	for _, plain := range []string{
		"",
		"plain",
		`one\backslash`,
		`two\\backslashes`,
		`a "quoted" name`,
		"tab\tand\nnewline",
		`C:\Program Files (x86)\Steam\steam.exe`,
		`ends with a backslash\`,
	} {
		escaped := Escape(plain)
		if got := Unescape(escaped); got != plain {
			t.Errorf("Unescape(Escape(%q)) = %q, want %q (escaped: %q)", plain, got, plain, escaped)
		}
	}
}

// An escape Valve does not define is kept as found: dropping the backslash
// would rewrite a value we are only passing through.
func TestUnescapeLeavesUndefinedSequencesAlone(t *testing.T) {
	t.Parallel()
	const raw = `steamapps\sourcemods`
	if got := Unescape(raw); got != raw {
		t.Errorf("Unescape(%q) = %q, want it unchanged", raw, got)
	}
}

// steamvdf hands back the raw bytes between the quotes, still escaped.
func TestReadBytesResolvesEscapes(t *testing.T) {
	t.Parallel()
	kv, err := ReadBytes([]byte("\"root\"\n{\n\t\"path\"\t\t\"a\\\\b\"\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(kv.Children) != 1 {
		t.Fatalf("children = %d, want 1", len(kv.Children))
	}
	if got, want := kv.Children[0].Value, `a\b`; got != want {
		t.Errorf("value = %q, want %q", got, want)
	}
}
