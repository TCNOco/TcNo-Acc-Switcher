package steam

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/paths"
)

type compactRecord struct {
	id   uint64
	name string
}

func encodeCompactAppArray(records []compactRecord) []byte {
	var buf bytes.Buffer
	buf.WriteString(compactAppArrayMagic)
	buf.WriteByte(compactAppArrayVersion)
	buf.Write(uvarint(uint64(len(records))))
	var prev uint64
	for _, r := range records {
		buf.Write(uvarint(r.id - prev))
		buf.Write(uvarint(uint64(len(r.name))))
		buf.WriteString(r.name)
		prev = r.id
	}
	return buf.Bytes()
}

func uvarint(v uint64) []byte {
	b := make([]byte, binary.MaxVarintLen64)
	return b[:binary.PutUvarint(b, v)]
}

func validCompactAppArray() []byte {
	return encodeCompactAppArray([]compactRecord{
		{10, "Ten"},
		{440, "Team Fortress 2"},
		{730, "Counter-Strike 2"},
		{100000, ""},
	})
}

func TestDecodeCompactAppArrayRoundTrip(t *testing.T) {
	m, err := decodeCompactAppArray(validCompactAppArray())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"10": "Ten", "440": "Team Fortress 2", "730": "Counter-Strike 2", "100000": ""}
	if len(m) != len(want) {
		t.Fatalf("got %d entries, want %d", len(m), len(want))
	}
	for id, name := range want {
		if m[id] != name {
			t.Fatalf("id %s: got %q, want %q", id, m[id], name)
		}
	}
}

func TestDecodeCompactAppArrayRejects(t *testing.T) {
	valid := validCompactAppArray()

	nonAscending := func() []byte {
		var buf bytes.Buffer
		buf.WriteString(compactAppArrayMagic)
		buf.WriteByte(compactAppArrayVersion)
		buf.Write(uvarint(2))
		buf.Write(uvarint(730))
		buf.Write(uvarint(0))
		buf.Write(uvarint(0)) // second record repeats id 730
		buf.Write(uvarint(0))
		return buf.Bytes()
	}()

	nameLenPastEnd := func() []byte {
		var buf bytes.Buffer
		buf.WriteString(compactAppArrayMagic)
		buf.WriteByte(compactAppArrayVersion)
		buf.Write(uvarint(1))
		buf.Write(uvarint(730))
		buf.Write(uvarint(64))
		buf.WriteString("short")
		return buf.Bytes()
	}()

	hostileCount := func() []byte {
		var buf bytes.Buffer
		buf.WriteString(compactAppArrayMagic)
		buf.WriteByte(compactAppArrayVersion)
		buf.Write(uvarint(compactAppArrayMaxCount + 1))
		return buf.Bytes()
	}()

	// A count the body cannot back must not be trusted for the allocation either.
	overlongCount := func() []byte {
		var buf bytes.Buffer
		buf.WriteString(compactAppArrayMagic)
		buf.WriteByte(compactAppArrayVersion)
		buf.Write(uvarint(compactAppArrayMaxCount))
		buf.Write(uvarint(730))
		buf.Write(uvarint(0))
		return buf.Bytes()
	}()

	badVersion := append([]byte(nil), valid...)
	badVersion[len(compactAppArrayMagic)] = 2

	longName := append([]byte(nil), []byte(compactAppArrayMagic)...)
	longName = append(longName, compactAppArrayVersion)
	longName = append(longName, uvarint(1)...)
	longName = append(longName, uvarint(730)...)
	longName = append(longName, uvarint(compactAppArrayMaxNameLen+1)...)
	longName = append(longName, bytes.Repeat([]byte("a"), compactAppArrayMaxNameLen+1)...)

	cases := []struct {
		name string
		raw  []byte
		want string
	}{
		{"empty", nil, "truncated header"},
		{"bad magic", append([]byte("XXXX"), valid[4:]...), "bad magic"},
		{"unknown version", badVersion, "unsupported version 2"},
		{"truncated body", valid[:len(valid)-6], "runs past end"},
		{"truncated header", valid[:3], "truncated header"},
		{"non-ascending id", nonAscending, "non-ascending"},
		{"name length past end", nameLenPastEnd, "runs past end"},
		{"hostile count", hostileCount, "record count"},
		{"count beyond body", overlongCount, "bad id delta"},
		{"name too long", longName, "name length"},
		{"trailing bytes", append(append([]byte(nil), valid...), 0, 0), "trailing bytes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := decodeCompactAppArray(tc.raw)
			if err == nil {
				t.Fatalf("expected error, got %d entries", len(m))
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func gzipForTest(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// The plain JSON mirror is the only rung left under the compact endpoint, so it
// has to carry the app on its own whenever the compact one is unusable.
func TestDownloadAppNameMapFallsBackToJSONMirror(t *testing.T) {
	cases := []struct {
		name    string
		compact func(w http.ResponseWriter, r *http.Request)
	}{
		{"missing", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }},
		{"not gzip", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("not gzip at all")) }},
		{"bad magic", func(w http.ResponseWriter, r *http.Request) {
			w.Write(gzipForTest(t, []byte("not a compact app array")))
		}},
		{"truncated body", func(w http.ResponseWriter, r *http.Request) {
			full := gzipForTest(t, validCompactAppArray())
			w.Write(full[:len(full)-8])
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			paths.ResetForTest(t.TempDir())
			t.Cleanup(func() { setSteamAppNameMapMemory(nil) })

			var hits []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits = append(hits, r.URL.Path)
				switch r.URL.Path {
				case "/compact":
					tc.compact(w, r)
				case "/json":
					w.Write(validSteamAppArrayJSON())
				}
			}))
			defer srv.Close()

			restore := steamAppNameMapSources
			steamAppNameMapSources = []steamAppNameMapSource{
				{url: srv.URL + "/compact", codec: appNameMapCodecGzip, format: appNameMapFormatCompact},
				{url: srv.URL + "/json", codec: appNameMapCodecPlain, format: appNameMapFormatJSON},
			}
			t.Cleanup(func() { steamAppNameMapSources = restore })

			if err := downloadAndStoreAppNameMap(context.Background(), "test"); err != nil {
				t.Fatal(err)
			}
			if len(hits) != 2 {
				t.Fatalf("expected both sources tried, got %v", hits)
			}
			m, err := getSteamAppNameMapCached()
			if err != nil {
				t.Fatal(err)
			}
			if m["730"] != "Counter-Strike 2" {
				t.Fatalf("unexpected name from the JSON mirror: %q", m["730"])
			}
		})
	}
}

func TestDownloadAppNameMapPrefersCompact(t *testing.T) {
	paths.ResetForTest(t.TempDir())
	t.Cleanup(func() {
		setSteamAppNameMapMemory(nil)
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/compact" {
			t.Errorf("unexpected request for %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Write(gzipForTest(t, validCompactAppArray()))
	}))
	defer srv.Close()

	restore := steamAppNameMapSources
	steamAppNameMapSources = []steamAppNameMapSource{
		{url: srv.URL + "/compact", codec: appNameMapCodecGzip, format: appNameMapFormatCompact},
		{url: srv.URL + "/json", codec: appNameMapCodecPlain, format: appNameMapFormatJSON},
	}
	t.Cleanup(func() { steamAppNameMapSources = restore })

	if err := downloadAndStoreAppNameMap(context.Background(), "test"); err != nil {
		t.Fatal(err)
	}
	m, err := getSteamAppNameMapCached()
	if err != nil {
		t.Fatal(err)
	}
	if m["440"] != "Team Fortress 2" {
		t.Fatalf("unexpected name from compact source: %q", m["440"])
	}
}

// A blocking on-demand download is uncancellable from the UI, so its total wait
// must not scale with the number of sources tried.
func TestEnsureAppNameMapBoundsBlockingDownload(t *testing.T) {
	paths.ResetForTest(t.TempDir())
	t.Cleanup(func() { setSteamAppNameMapMemory(nil) })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	restore := steamAppNameMapSources
	steamAppNameMapSources = nil
	for i := 0; i < 8; i++ {
		steamAppNameMapSources = append(steamAppNameMapSources, steamAppNameMapSource{
			url: srv.URL, codec: appNameMapCodecPlain, format: appNameMapFormatJSON,
		})
	}
	t.Cleanup(func() { steamAppNameMapSources = restore })

	restoreTimeout := steamAppNameMapOnDemandTimeout
	steamAppNameMapOnDemandTimeout = 300 * time.Millisecond
	t.Cleanup(func() { steamAppNameMapOnDemandTimeout = restoreTimeout })

	start := time.Now()
	if _, err := ensureAppNameMap(context.Background()); err == nil {
		t.Fatal("expected the blocking download to fail")
	}
	if elapsed := time.Since(start); elapsed > 4*steamAppNameMapOnDemandTimeout {
		t.Fatalf("waited %v for %d sources; the deadline covers one rung, not the walk", elapsed, len(steamAppNameMapSources))
	}
}
