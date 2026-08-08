package vaultrecord

import "testing"

func TestSniffTreatsAnEnvelopeLessObjectAsAMaFile(t *testing.T) {
	t.Parallel()
	// An SDA maFile has no schema envelope at all, and mafile.ParsePlaintext
	// rejects anything that does carry one, so "no kind" is the discriminator.
	raw := []byte(`{"shared_secret":"AAAA","identity_secret":"BBBB","device_id":"android:x"}`)
	if got := Sniff(raw); got != KindMaFile {
		t.Fatalf("Sniff = %v, want KindMaFile", got)
	}
}

func TestSniffReadsTheKnownKinds(t *testing.T) {
	t.Parallel()
	cases := map[string]Kind{
		`{"kind":"steamguard-login-only","version":1}`:         KindLoginOnly,
		`{"kind":"steamguard-enrollment-pending","version":2}`: KindEnrollmentPending,
		`{"kind":"steamguard-something-newer","version":9}`:    KindUnknown,
	}
	for raw, want := range cases {
		if got := Sniff([]byte(raw)); got != want {
			t.Fatalf("Sniff(%s) = %v, want %v", raw, got, want)
		}
	}
}

func TestSniffRejectsAnythingThatIsNotAJSONObject(t *testing.T) {
	t.Parallel()
	// `null` is the trap: it decodes cleanly into the marker struct and would
	// otherwise be reported as a maFile purely for lacking a "kind".
	for _, raw := range []string{"", "   ", "null", `"a string"`, "123", "[]", "{", `{"kind":`, "not json"} {
		if got := Sniff([]byte(raw)); got != KindUnknown {
			t.Fatalf("Sniff(%q) = %v, want KindUnknown", raw, got)
		}
	}
}

func TestSniffRejectsTrailingContent(t *testing.T) {
	t.Parallel()
	if got := Sniff([]byte(`{"kind":"steamguard-login-only"} {"kind":"steamguard-login-only"}`)); got != KindUnknown {
		t.Fatalf("Sniff = %v, want KindUnknown", got)
	}
}

func TestSniffRejectsAnOversizedRecord(t *testing.T) {
	t.Parallel()
	raw := make([]byte, maxSniffBytes+1)
	for i := range raw {
		raw[i] = ' '
	}
	copy(raw, []byte(`{"kind":"steamguard-login-only"}`))
	if got := Sniff(raw); got != KindUnknown {
		t.Fatalf("Sniff = %v, want KindUnknown", got)
	}
}
