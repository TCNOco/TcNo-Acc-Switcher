package sqliteread

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixture = "testdata/login_cache.db"

// copyFixture returns a private copy of the fixture, optionally mutated.
func copyFixture(t *testing.T, mutate func([]byte) []byte) string {
	t.Helper()
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if mutate != nil {
		data = mutate(data)
	}
	path := filepath.Join(t.TempDir(), "CachedData.db")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write copy: %v", err)
	}
	return path
}

func TestQueryScalar(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  string
	}{
		// The Battle.net descriptor's shape: quoted literal against an INTEGER column.
		{"integer affinity", `SELECT battle_tag FROM login_cache WHERE account_id_lo = '1111185900'`, "Player0#1000"},
		{"unquoted literal", `SELECT battle_tag FROM login_cache WHERE account_id_lo = 1111185901`, "Player1#1001"},
		{"text column", `SELECT account_id_lo FROM login_cache WHERE battle_tag = 'Player2#1002'`, "1111185902"},
		{"row on a later leaf page", `SELECT battle_tag FROM login_cache WHERE account_id_lo = '1111185959'`, "Player59#1059"},
		{"overflow payload", `SELECT battle_tag FROM login_cache WHERE account_id_lo = '9999999999'`, strings.Repeat("L", 3000)},
		{"null value", `SELECT battle_tag FROM login_cache WHERE account_id_lo = '4242'`, ""},
		{"no match", `SELECT battle_tag FROM login_cache WHERE account_id_lo = '5'`, ""},
		{"rowid alias column", `SELECT tag FROM alias_tbl WHERE id = '7'`, "Aliased#0007"},
		{"real value", `SELECT v FROM other WHERE k = 'pi'`, "3.5"},
		{"blob value", `SELECT v FROM other WHERE k = 'blob'`, "\xde\xad\xbe\xef"},
		{"case insensitive keywords", `select battle_tag from login_cache where account_id_lo = '1111185900'`, "Player0#1000"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := QueryScalar(fixture, c.query)
			if err != nil {
				t.Fatalf("QueryScalar: %v", err)
			}
			if got != c.want {
				t.Fatalf("got %q want %q", truncate(got), truncate(c.want))
			}
		})
	}
}

func TestQueryScalarRejectsUnsupportedQueries(t *testing.T) {
	for _, q := range []string{
		`SELECT * FROM login_cache WHERE account_id_lo = '4242'`,
		`SELECT battle_tag FROM login_cache`,
		`SELECT battle_tag FROM login_cache WHERE account_id_lo = "4242"`,
		`SELECT battle_tag FROM login_cache WHERE account_id_lo LIKE '4242'`,
		`SELECT a.battle_tag FROM login_cache a JOIN b ON b.id = a.id WHERE a.id = '1'`,
		`SELECT battle_tag FROM login_cache WHERE account_id_lo = '4242'; DROP TABLE login_cache`,
		`SELECT missing FROM login_cache WHERE account_id_lo = '4242'`,
		`SELECT battle_tag FROM missing WHERE account_id_lo = '4242'`,
		// The reader has no row limit to apply, so it must refuse rather than
		// answer as though LIMIT meant something.
		`SELECT battle_tag FROM login_cache WHERE account_id_lo = '4242' LIMIT 0`,
		`SELECT battle_tag FROM login_cache WHERE account_id_lo = '1111185900' LIMIT 2`,
	} {
		if got, err := QueryScalar(fixture, q); err == nil {
			t.Fatalf("query %q unexpectedly succeeded with %q", q, got)
		}
	}
}

// A database in WAL mode may hold the current row only in its sidecar; returning
// the main file's older value would switch to the wrong account.
func TestQueryScalarWAL(t *testing.T) {
	const q = `SELECT battle_tag FROM login_cache WHERE account_id_lo = '1111185900'`
	cases := []struct {
		name    string
		wal     []byte
		wantErr bool
	}{
		{"no sidecar", nil, false},
		{"frames the database may not have", buildWAL(0x377f0682, 512, true), true},
		{"leftovers from a checkpoint", buildWAL(0x377f0682, 512, false), false},
		{"unrecognisable sidecar", make([]byte, 512), true},
		{"sidecar for a different page size", buildWAL(0x377f0682, 4096, true), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := copyFixture(t, func(b []byte) []byte {
				b[19] = 2
				return b
			})
			if c.wal != nil {
				if err := os.WriteFile(path+"-wal", c.wal, 0o600); err != nil {
					t.Fatalf("write wal: %v", err)
				}
			}
			got, err := QueryScalar(path, q)
			if c.wantErr {
				if !errors.Is(err, ErrWAL) || !errors.Is(err, ErrPendingWrites) {
					t.Fatalf("got %q, %v; want ErrWAL", got, err)
				}
				return
			}
			if err != nil || got != "Player0#1000" {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}

// buildWAL writes a write-ahead log holding one frame, whose salt either matches
// the log header (data the main database may not have) or does not (frames left
// behind by a checkpoint).
func buildWAL(magic uint32, pageSize int, live bool) []byte {
	b := make([]byte, 32+24+pageSize)
	binary.BigEndian.PutUint32(b[0:4], magic)
	binary.BigEndian.PutUint32(b[4:8], 3007000)
	binary.BigEndian.PutUint32(b[8:12], uint32(pageSize))
	copy(b[16:24], []byte{1, 2, 3, 4, 5, 6, 7, 8})
	binary.BigEndian.PutUint32(b[32:36], 2)
	binary.BigEndian.PutUint32(b[36:40], 24)
	if live {
		copy(b[40:48], b[16:24])
	}
	return b
}

// A rollback journal left behind by an interrupted write means the main file's
// pages are the ones the journal exists to undo. Battle.net's CachedData.db is
// journal_mode=delete, so this is the sidecar the reader actually meets.
func TestQueryScalarRollbackJournal(t *testing.T) {
	const q = `SELECT battle_tag FROM login_cache WHERE account_id_lo = '1111185900'`
	cases := []struct {
		name    string
		journal []byte
		wantErr bool
	}{
		{"no sidecar", nil, false},
		{"a transaction the database still owes", buildJournal(0xd9d505f920a163d7, 512), true},
		{"truncated to nothing after commit", []byte{}, false},
		{"header zeroed after commit", make([]byte, 512), false},
		{"shorter than a header", buildJournal(0xd9d505f920a163d7, 512)[:20], false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := copyFixture(t, nil)
			if c.journal != nil {
				if err := os.WriteFile(path+"-journal", c.journal, 0o600); err != nil {
					t.Fatalf("write journal: %v", err)
				}
			}
			got, err := QueryScalar(path, q)
			if c.wantErr {
				if !errors.Is(err, ErrJournal) || !errors.Is(err, ErrPendingWrites) {
					t.Fatalf("got %q, %v; want ErrJournal", got, err)
				}
				return
			}
			if err != nil || got != "Player0#1000" {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}

// buildJournal writes a rollback journal header for one page.
func buildJournal(magic uint64, pageSize int) []byte {
	b := make([]byte, 28+pageSize)
	binary.BigEndian.PutUint64(b[0:8], magic)
	binary.BigEndian.PutUint32(b[8:12], 1)
	binary.BigEndian.PutUint32(b[16:20], 25)
	binary.BigEndian.PutUint32(b[20:24], 512)
	binary.BigEndian.PutUint32(b[24:28], uint32(pageSize))
	return b
}

func TestQueryScalarRejectsCorruptDatabases(t *testing.T) {
	const q = `SELECT battle_tag FROM login_cache WHERE account_id_lo = '9999999999'`
	cases := []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"truncated file", func(b []byte) []byte { return b[:300] }},
		{"bad magic", func(b []byte) []byte { b[0] = 'X'; return b }},
		{"bad page size", func(b []byte) []byte { b[16], b[17] = 0, 3; return b }},
		{"root page beyond file", func(b []byte) []byte { return setRootPage(b, 0x7E) }},
		{"root page zero", func(b []byte) []byte { return setRootPage(b, 0) }},
		{"cell pointer past page end", func(b []byte) []byte { b[108], b[109] = 0xFF, 0xF0; return b }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, err := QueryScalar(copyFixture(t, c.mutate), q); err == nil {
				t.Fatalf("corrupt database unexpectedly succeeded with %q", truncate(got))
			}
		})
	}
}

// A malformed file must never panic, read out of range, or hand back a value
// that is not in the database: this parses input the app does not control, in a
// process installed with admin rights.
func TestQueryScalarNeverInventsAValue(t *testing.T) {
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	want := strings.Repeat("L", 3000)
	for n := 0; n < len(data); n += 97 {
		path := filepath.Join(t.TempDir(), "t.db")
		if err := os.WriteFile(path, data[:n], 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		got, err := QueryScalar(path, `SELECT battle_tag FROM login_cache WHERE account_id_lo = '9999999999'`)
		if err == nil && got != "" && got != want {
			t.Fatalf("truncated to %d bytes: got %q", n, truncate(got))
		}
	}
}

// setRootPage rewrites login_cache's rootpage in the schema record on page 1.
func setRootPage(b []byte, page byte) []byte {
	i := indexRootPageField(b)
	if i < 0 {
		panic("login_cache schema row not found in fixture")
	}
	b[i] = page
	return b
}

// indexRootPageField finds the one-byte rootpage field of the login_cache
// schema record, which sits directly before its CREATE TABLE text.
func indexRootPageField(b []byte) int {
	i := strings.Index(string(b), "CREATE TABLE login_cache")
	if i < 0 {
		return -1
	}
	return i - 1
}

func truncate(s string) string {
	if len(s) > 60 {
		return s[:60] + "..."
	}
	return s
}
