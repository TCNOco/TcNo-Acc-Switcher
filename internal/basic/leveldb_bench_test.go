package basic

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"TcNo-Acc-Switcher/internal/platform"

	"github.com/syndtr/goleveldb/leveldb"
)

// benchLevelDBKey is the shape Discord's Local Storage uses for the blob the
// switcher reads the account id out of.
const benchLevelDBKey = "_https://discord.comMultiAccountStore"

// seedBenchLevelDB writes a database roughly the size of a real Discord Local
// Storage: the account blob plus a few hundred other keys, so opening it costs
// what opening the real one costs.
func seedBenchLevelDB(tb testing.TB) string {
	tb.Helper()
	dbPath := filepath.Join(tb.TempDir(), "leveldb")
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		tb.Fatalf("open leveldb: %v", err)
	}

	blob := `{"_state":{"users":[{"id":"224612345678901234","avatar":"a_1b2c3d4e5f","username":"bench"}]}}`
	if err := db.Put([]byte(benchLevelDBKey), []byte(blob), nil); err != nil {
		tb.Fatalf("put account blob: %v", err)
	}
	filler := strings.Repeat("x", 512)
	for i := 0; i < 300; i++ {
		k := fmt.Appendf(nil, "_https://discord.comSetting%03d", i)
		if err := db.Put(k, []byte(filler), nil); err != nil {
			tb.Fatalf("put filler: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		tb.Fatalf("close leveldb: %v", err)
	}
	return dbPath
}

// BenchmarkLevelDBReadPerAccount is the shape a Discord account page takes:
// descriptor variable resolution resolves a leveldb reference per account, and
// the operation ends with closeSharedLevelDBHandles.
//
// Fresh reopens the database for every reference. Cached opens it once for the
// whole operation and hands the same handle to each read, which is what the
// begin/end closeSharedLevelDBHandles calls around every flow phase already
// assume. On Windows the gap is wider than this: with the platform running its
// LOCK forces the open down the temp-copy path, which byte-copies the entire
// Local Storage directory - once per read on the fresh path.
func BenchmarkLevelDBReadPerAccount(b *testing.B) {
	for _, accounts := range []int{1, 10, 50} {
		dbPath := seedBenchLevelDB(b)
		ref := "leveldb:" + dbPath + ":" + benchLevelDBKey + ":_state.users.0.id"
		parsed, err := parseLevelDBReference(ref)
		if err != nil {
			b.Fatalf("parse reference: %v", err)
		}
		expanded := platform.ExpandPathTokens(platform.ExpandWindowsPath(parsed.Path), platform.PathTokenContext{})

		b.Run(fmt.Sprintf("%daccounts/Fresh", accounts), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for a := 0; a < accounts; a++ {
					if _, err := sharedLevelDBStore.readValueFresh(expanded, parsed.Key, parsed.JSONPath); err != nil {
						b.Fatal(err)
					}
				}
				closeSharedLevelDBHandles("bench")
			}
		})

		b.Run(fmt.Sprintf("%daccounts/Cached", accounts), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for a := 0; a < accounts; a++ {
					if _, err := sharedLevelDBStore.readValue(expanded, parsed.Key, parsed.JSONPath); err != nil {
						b.Fatal(err)
					}
				}
				closeSharedLevelDBHandles("bench")
			}
		})
	}
}
