package accountstore

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/paths"
)

const (
	idA = "76561198000000100"
	idB = "76561198000000200"
)

func useTempRoot(t *testing.T) {
	t.Helper()
	paths.ResetForTest(t.TempDir())
}

func mustLoad(t *testing.T) []Record {
	t.Helper()
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return got
}

func writeRaw(t *testing.T, body string) string {
	t.Helper()
	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	useTempRoot(t)
	got := mustLoad(t)
	if len(got) != 0 {
		t.Fatalf("want no records, got %d", len(got))
	}
}

// The vdf sync runs on every window focus. If an unchanged sync reported a
// change it would rewrite the file every few seconds.
func TestUpsertManyReportsChangeOnlyWhenSomethingMoved(t *testing.T) {
	useTempRoot(t)
	rec := Record{SteamID64: idA, AccountName: "acct_a", PersonaName: "A", Source: SourceVDF}

	res, err := UpsertMany([]Record{rec})
	if err != nil {
		t.Fatalf("first UpsertMany: %v", err)
	}
	if !res.Changed {
		t.Fatal("inserting a new account should report a change")
	}

	res, err = UpsertMany([]Record{rec})
	if err != nil {
		t.Fatalf("second UpsertMany: %v", err)
	}
	if res.Changed {
		t.Fatal("re-syncing an identical account should not report a change")
	}
}

// The Steam list build counts accounts it has not seen by diffing Before
// against After, so a merge has to report both sides of itself rather than just
// the result.
func TestUpsertManyReportsBothSidesOfTheMerge(t *testing.T) {
	useTempRoot(t)
	first := Record{SteamID64: idA, AccountName: "acct_a", Source: SourceVDF}
	second := Record{SteamID64: idB, AccountName: "acct_b", Source: SourceVDF}

	if _, err := UpsertMany([]Record{first}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	res, err := UpsertMany([]Record{first, second})
	if err != nil {
		t.Fatalf("UpsertMany: %v", err)
	}
	if !res.Changed {
		t.Fatal("adding a second account should report a change")
	}
	if len(res.Before) != 1 || res.Before[0].SteamID64 != idA {
		t.Fatalf("Before = %#v, want only %s", res.Before, idA)
	}
	if len(res.After) != 2 {
		t.Fatalf("After has %d records, want 2", len(res.After))
	}
}

// An empty merge still has to say what the store holds: the caller uses that
// instead of a Load of its own.
func TestUpsertManyWithNoRecordsStillReportsTheStore(t *testing.T) {
	useTempRoot(t)
	if _, err := Upsert(Record{SteamID64: idA, AccountName: "acct_a", Source: SourceVDF}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	res, err := UpsertMany(nil)
	if err != nil {
		t.Fatalf("UpsertMany(nil): %v", err)
	}
	if res.Changed {
		t.Fatal("an empty merge must not report a change")
	}
	if len(res.After) != 1 || res.After[0].SteamID64 != idA {
		t.Fatalf("After = %#v, want the stored account", res.After)
	}
}

// A Steam Guard enrollment knows the ID and login name and nothing else. It
// must not blank the persona and last-login date a vdf sync recorded.
func TestUpsertEmptyFieldsPreserveStoredValues(t *testing.T) {
	useTempRoot(t)
	if _, err := Upsert(Record{
		SteamID64:   idA,
		AccountName: "acct_a",
		PersonaName: "Persona A",
		Timestamp:   "1700000000",
		Source:      SourceVDF,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := Upsert(Record{SteamID64: idA, AccountName: "acct_a", Source: SourceSteamGuard}); err != nil {
		t.Fatal(err)
	}

	got, ok, err := Get(idA)
	if err != nil || !ok {
		t.Fatalf("Get: %v ok=%v", err, ok)
	}
	if got.PersonaName != "Persona A" {
		t.Errorf("PersonaName = %q, want it preserved", got.PersonaName)
	}
	if got.Timestamp != "1700000000" {
		t.Errorf("Timestamp = %q, want it preserved", got.Timestamp)
	}
	if got.Source != SourceVDF {
		t.Errorf("Source = %q, want the first-seen provenance kept", got.Source)
	}
}

func TestUpsertNonEmptyFieldsOverwrite(t *testing.T) {
	useTempRoot(t)
	if _, err := Upsert(Record{SteamID64: idA, PersonaName: "Old", Source: SourceVDF}); err != nil {
		t.Fatal(err)
	}
	if _, err := Upsert(Record{SteamID64: idA, PersonaName: "New", Timestamp: "1700000001"}); err != nil {
		t.Fatal(err)
	}

	got, _, err := Get(idA)
	if err != nil {
		t.Fatal(err)
	}
	if got.PersonaName != "New" {
		t.Errorf("PersonaName = %q, want New", got.PersonaName)
	}
	if got.Timestamp != "1700000001" {
		t.Errorf("Timestamp = %q, want 1700000001", got.Timestamp)
	}
}

func TestRemove(t *testing.T) {
	useTempRoot(t)
	if _, err := UpsertMany([]Record{
		{SteamID64: idA, AccountName: "a", Source: SourceVDF},
		{SteamID64: idB, AccountName: "b", Source: SourceVDF},
	}); err != nil {
		t.Fatal(err)
	}
	if err := Remove(idA); err != nil {
		t.Fatal(err)
	}
	got := mustLoad(t)
	if len(got) != 1 || got[0].SteamID64 != idB {
		t.Fatalf("after Remove got %+v, want only %s", got, idB)
	}
	// Removing an account that is not there is not an error: Forget runs against
	// the store and the vdf independently, and either may already lack the row.
	if err := Remove(idA); err != nil {
		t.Fatalf("removing an absent account: %v", err)
	}
}

// A store the app cannot parse must be left alone rather than replaced. It is
// the only copy of accounts Steam has forgotten.
func TestCorruptStoreErrorsAndIsNotOverwritten(t *testing.T) {
	useTempRoot(t)
	const body = `{"version":1,"entries":[ THIS IS NOT JSON`
	path := writeRaw(t, body)

	if _, err := Load(); err == nil {
		t.Fatal("Load should reject a malformed store")
	}
	if _, err := UpsertMany([]Record{{SteamID64: idA, Source: SourceVDF}}); err == nil {
		t.Fatal("UpsertMany should refuse to write over a malformed store")
	}
	if err := Remove(idA); err == nil {
		t.Fatal("Remove should refuse to write over a malformed store")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != body {
		t.Fatalf("malformed store was rewritten:\n%s", raw)
	}
}

func TestLoadRejectsUnknownVersion(t *testing.T) {
	useTempRoot(t)
	writeRaw(t, `{"version":99,"entries":[]}`)
	if _, err := Load(); err == nil {
		t.Fatal("a future store version should be rejected, not read as empty")
	}
}

// Additive fields must not break an older build, so unknown keys are tolerated.
func TestLoadToleratesUnknownFields(t *testing.T) {
	useTempRoot(t)
	writeRaw(t, `{"version":1,"entries":[{"steamId64":"`+idA+`","accountName":"a","somethingNew":true}]}`)
	got := mustLoad(t)
	if len(got) != 1 || got[0].AccountName != "a" {
		t.Fatalf("got %+v, want the record read despite the unknown field", got)
	}
}

func TestLoadSkipsRowsThatCouldNeverBeSwitchedTo(t *testing.T) {
	useTempRoot(t)
	writeRaw(t, `{"version":1,"entries":[
		{"steamId64":"not-an-id","accountName":"junk"},
		{"steamId64":"`+idA+`","accountName":"good"}
	]}`)
	got := mustLoad(t)
	if len(got) != 1 || got[0].SteamID64 != idA {
		t.Fatalf("got %+v, want only the valid row", got)
	}
}

func TestMergeRecordsFirstSeenIsWriteOnce(t *testing.T) {
	stored := []Record{{SteamID64: idA, FirstSeen: 1000, Source: SourceVDF}}
	out, changed := mergeRecords(stored, []Record{{SteamID64: idA, Source: SourceVDF}}, 1000)
	if changed {
		t.Fatal("a same-second re-sync should not report a change")
	}
	if out[0].FirstSeen != 1000 {
		t.Errorf("FirstSeen = %d, want 1000", out[0].FirstSeen)
	}
}

func TestMergeRecordsLastSeenHonoursGranularity(t *testing.T) {
	const base = int64(1_700_000_000)
	stored := []Record{{SteamID64: idA, FirstSeen: base, LastSeenInVDF: base, Source: SourceVDF}}
	incoming := []Record{{SteamID64: idA, Source: SourceVDF}}

	within := base + int64(lastSeenGranularity/time.Second) - 1
	if _, changed := mergeRecords(stored, incoming, within); changed {
		t.Error("a sighting inside the granularity window should not rewrite the file")
	}

	after := base + int64(lastSeenGranularity/time.Second)
	out, changed := mergeRecords(stored, incoming, after)
	if !changed {
		t.Fatal("a sighting past the granularity window should bump LastSeenInVDF")
	}
	if out[0].LastSeenInVDF != after {
		t.Errorf("LastSeenInVDF = %d, want %d", out[0].LastSeenInVDF, after)
	}
}

// A vault-only account has never been in loginusers.vdf, and LastSeenInVDF is
// what says so.
func TestMergeRecordsSteamGuardSourceLeavesLastSeenZero(t *testing.T) {
	out, changed := mergeRecords(nil, []Record{{SteamID64: idA, AccountName: "a", Source: SourceSteamGuard}}, 1000)
	if !changed || len(out) != 1 {
		t.Fatalf("changed=%v out=%+v", changed, out)
	}
	if out[0].LastSeenInVDF != 0 {
		t.Errorf("LastSeenInVDF = %d, want 0 for a vault-only account", out[0].LastSeenInVDF)
	}
	if out[0].FirstSeen != 1000 {
		t.Errorf("FirstSeen = %d, want 1000", out[0].FirstSeen)
	}
}

func TestSaveRejectsOverfullStore(t *testing.T) {
	useTempRoot(t)
	records := make([]Record, 0, maxEntries+1)
	for i := 0; i <= maxEntries; i++ {
		records = append(records, Record{
			SteamID64: strconv.FormatUint(76561197960265728+uint64(i), 10),
			Source:    SourceVDF,
		})
	}
	if err := Save(records); err == nil {
		t.Fatal("saving more than maxEntries should fail loudly, not truncate")
	}
}
