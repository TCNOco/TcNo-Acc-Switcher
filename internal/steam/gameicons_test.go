package steam

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// failingIconDoer fails the test if the icon path ever reaches the network.
type failingIconDoer struct{ calls atomic.Int32 }

func (d *failingIconDoer) Do(*http.Request) (*http.Response, error) {
	d.calls.Add(1)
	return nil, errors.New("network must not be used here")
}

// useTempIconCache points the icon cache at a temp dir and swaps in a client that
// treats any request as a failure. Returns the cache dir.
func useTempIconCache(t *testing.T) (string, *failingIconDoer) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "wwwroot", "img", "gameicons", "square")
	prevDir, prevClient := gameIconCacheDir, gameIconClient
	doer := &failingIconDoer{}
	gameIconCacheDir = func() (string, error) { return dir, nil }
	gameIconClient = doer
	t.Cleanup(func() {
		gameIconCacheDir, gameIconClient = prevDir, prevClient
	})
	return dir, doer
}

// useLibraryCache pins the memoised librarycache lookup at dir, so a test never
// resolves art out of the machine's own Steam install.
func useLibraryCache(t *testing.T, dir string) {
	t.Helper()
	gameIconLibraryMu.Lock()
	prevDir, prevTime := gameIconLibraryDir, gameIconLibraryTime
	gameIconLibraryDir, gameIconLibraryTime = dir, time.Now()
	gameIconLibraryMu.Unlock()
	t.Cleanup(func() {
		gameIconLibraryMu.Lock()
		gameIconLibraryDir, gameIconLibraryTime = prevDir, prevTime
		gameIconLibraryMu.Unlock()
	})
}

func jpegBytes() []byte {
	return append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, []byte("jfif-ish payload")...)
}

// encodeJPEG builds a real JPEG. The icon path measures dimensions, so a stub
// carrying only the magic bytes would not decode.
func encodeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.Gray{Y: 200})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func squareJPEG(t *testing.T, size int) []byte {
	t.Helper()
	return encodeJPEG(t, size, size)
}

func writeIconFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeAppID(t *testing.T) {
	good := map[string]string{
		"730":        "730",
		" 440 ":      "440",
		"1":          "1",
		"4294967295": "4294967295",
	}
	for in, want := range good {
		got, ok := normalizeAppID(in)
		if !ok || got != want {
			t.Fatalf("normalizeAppID(%q) = %q, %v; want %q, true", in, got, ok, want)
		}
	}

	bad := []string{
		"",
		"   ",
		"..",
		"../../windows/system32/config",
		"730/../../evil",
		`730\..\evil`,
		"730.jpg",
		"73 0",
		"0",
		"0730",
		"-730",
		"+730",
		"7e3",
		"abc",
		"730%2f",
		"12345678901", // 11 digits: past uint32
		strings.Repeat("9", 64),
	}
	for _, in := range bad {
		if got, ok := normalizeAppID(in); ok {
			t.Fatalf("normalizeAppID(%q) accepted, returned %q", in, got)
		}
	}
}

func TestGameIconURL(t *testing.T) {
	if got := GameIconURL(" 730 "); got != "/img/gameicons/square/730.jpg" {
		t.Fatalf("GameIconURL = %q", got)
	}
	if got := GameIconURL("../evil"); got != "" {
		t.Fatalf("GameIconURL(traversal) = %q, want empty", got)
	}
}

func TestEnsureGameIconCacheHitSkipsNetwork(t *testing.T) {
	dir, doer := useTempIconCache(t)
	writeIconFile(t, filepath.Join(dir, "730.jpg"), jpegBytes())

	url, err := EnsureGameIcon(context.Background(), "730")
	if err != nil {
		t.Fatal(err)
	}
	if url != "/img/gameicons/square/730.jpg" {
		t.Fatalf("url = %q", url)
	}
	if n := doer.calls.Load(); n != 0 {
		t.Fatalf("cache hit made %d HTTP requests", n)
	}
}

func TestEnsureGameIconIgnoresEmptyCacheFile(t *testing.T) {
	dir, _ := useTempIconCache(t)
	// A zero-byte file is what a half-finished write leaves behind; it must not
	// be reported as a hit or the view renders a permanently broken image.
	writeIconFile(t, filepath.Join(dir, "730.jpg"), nil)

	if isGameIconFile(filepath.Join(dir, "730.jpg")) {
		t.Fatal("zero-byte cache file counted as a hit")
	}
}

func TestEnsureGameIconRejectsMalformedAppID(t *testing.T) {
	dir, doer := useTempIconCache(t)

	url, err := EnsureGameIcon(context.Background(), "../../escape")
	if err == nil {
		t.Fatal("expected an error for a malformed app id")
	}
	if url != "" {
		t.Fatalf("url = %q, want empty", url)
	}
	if n := doer.calls.Load(); n != 0 {
		t.Fatalf("malformed id made %d HTTP requests", n)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Fatal("malformed id created the cache directory")
	}
}

func TestFindLibraryCacheIconPrefersSquareClientIcon(t *testing.T) {
	lc := t.TempDir()
	// Modern Steam: per-appid folder holding the <img_icon_url>.jpg client icon
	// beside the header banner and the hero and logo art.
	icon := strings.Repeat("a", 40) + ".jpg"
	writeIconFile(t, filepath.Join(lc, "730", icon), squareJPEG(t, 32))
	writeIconFile(t, filepath.Join(lc, "730", "header.jpg"), encodeJPEG(t, 460, 215))
	writeIconFile(t, filepath.Join(lc, "730", "library_hero.jpg"), jpegBytes())
	writeIconFile(t, filepath.Join(lc, "730", "logo.png"), jpegBytes())

	got := findLibraryCacheIcon(lc, "730")
	if want := filepath.Join(lc, "730", icon); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFindLibraryCacheIconPrefersTheSmallestSquareAsset(t *testing.T) {
	lc := t.TempDir()
	// A few dozen app folders hold a second, far larger square asset next to the
	// client icon. The list row wants the icon, not the poster.
	small := strings.Repeat("b", 40) + ".jpg"
	large := strings.Repeat("1", 40) + ".jpg"
	writeIconFile(t, filepath.Join(lc, "570", small), squareJPEG(t, 32))
	writeIconFile(t, filepath.Join(lc, "570", large), squareJPEG(t, 512))

	// The larger asset sorts first by name, so name order alone would pick it.
	got := findLibraryCacheIcon(lc, "570")
	if want := filepath.Join(lc, "570", small); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFindLibraryCacheIconIgnoresOffSquareAndUndecodableCandidates(t *testing.T) {
	lc := t.TempDir()
	// A hash-named file that is not actually square must not be promoted over the
	// header, and neither must one that does not decode at all.
	writeIconFile(t, filepath.Join(lc, "440", strings.Repeat("c", 40)+".jpg"), encodeJPEG(t, 64, 32))
	writeIconFile(t, filepath.Join(lc, "440", strings.Repeat("d", 40)+".jpg"), jpegBytes())
	writeIconFile(t, filepath.Join(lc, "440", "header.jpg"), encodeJPEG(t, 460, 215))

	got := findLibraryCacheIcon(lc, "440")
	if want := filepath.Join(lc, "440", "header.jpg"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFindLibraryCacheIconFallsBackToLocalisedHeader(t *testing.T) {
	lc := t.TempDir()
	// No square icon and no top-level header: newer clients park localised art in
	// sha1-named subfolders (observed on 730 and other multi-language titles).
	writeIconFile(t, filepath.Join(lc, "440", "library_hero.jpg"), jpegBytes())
	writeIconFile(t, filepath.Join(lc, "440", "ffff0000", "library_header.jpg"), jpegBytes())
	writeIconFile(t, filepath.Join(lc, "440", "0000ffff", "header.jpg"), jpegBytes())

	// header.jpg outranks library_header.jpg regardless of folder ordering.
	got := findLibraryCacheIcon(lc, "440")
	if want := filepath.Join(lc, "440", "0000ffff", "header.jpg"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFindLibraryCacheIconPrefersTopLevelOverSubfolder(t *testing.T) {
	lc := t.TempDir()
	writeIconFile(t, filepath.Join(lc, "570", "header.jpg"), jpegBytes())
	writeIconFile(t, filepath.Join(lc, "570", "aaaa1111", "header.jpg"), jpegBytes())

	got := findLibraryCacheIcon(lc, "570")
	if want := filepath.Join(lc, "570", "header.jpg"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFindLibraryCacheIconLegacyFlatLayout(t *testing.T) {
	lc := t.TempDir()
	// The pre-rework layout named the square icon outright, so it outranks the
	// flat header - but only once measured, since the name alone proves nothing.
	writeIconFile(t, filepath.Join(lc, "220_icon.jpg"), squareJPEG(t, 32))
	writeIconFile(t, filepath.Join(lc, "220_header.jpg"), jpegBytes())
	writeIconFile(t, filepath.Join(lc, "221_icon.jpg"), encodeJPEG(t, 460, 215))
	writeIconFile(t, filepath.Join(lc, "221_header.jpg"), jpegBytes())

	if got, want := findLibraryCacheIcon(lc, "220"), filepath.Join(lc, "220_icon.jpg"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got, want := findLibraryCacheIcon(lc, "221"), filepath.Join(lc, "221_header.jpg"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFindLibraryCacheIconMissing(t *testing.T) {
	lc := t.TempDir()
	writeIconFile(t, filepath.Join(lc, "730", "library_hero.jpg"), jpegBytes())
	writeIconFile(t, filepath.Join(lc, "730", "library_600x900.jpg"), jpegBytes())
	writeIconFile(t, filepath.Join(lc, "730", "logo.png"), jpegBytes())

	if got := findLibraryCacheIcon(lc, "730"); got != "" {
		t.Fatalf("hero, portrait and logo art should not be used as an icon, got %q", got)
	}
	if got := findLibraryCacheIcon(lc, "999999"); got != "" {
		t.Fatalf("unknown app id returned %q", got)
	}
	if got := findLibraryCacheIcon(filepath.Join(lc, "does-not-exist"), "730"); got != "" {
		t.Fatalf("absent librarycache returned %q", got)
	}
	if got := findLibraryCacheIcon("", "730"); got != "" {
		t.Fatalf("empty librarycache returned %q", got)
	}
	if got := findLibraryCacheIcon(lc, "../730"); got != "" {
		t.Fatalf("malformed app id returned %q", got)
	}
}

func TestEnsureGameIconCachesTheLocalSquareIcon(t *testing.T) {
	dir, doer := useTempIconCache(t)
	lc := t.TempDir()
	useLibraryCache(t, lc)
	icon := squareJPEG(t, 32)
	writeIconFile(t, filepath.Join(lc, "730", strings.Repeat("a", 40)+".jpg"), icon)
	writeIconFile(t, filepath.Join(lc, "730", "header.jpg"), encodeJPEG(t, 460, 215))

	url, err := EnsureGameIcon(context.Background(), "730")
	if err != nil {
		t.Fatal(err)
	}
	if url != "/img/gameicons/square/730.jpg" {
		t.Fatalf("url = %q", url)
	}
	cached, err := os.ReadFile(filepath.Join(dir, "730.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(cached, icon) {
		t.Fatal("cached file is not the square client icon")
	}
	if n := doer.calls.Load(); n != 0 {
		t.Fatalf("local icon made %d HTTP requests", n)
	}
}

func TestPruneLegacyGameIconsDropsTheOldHeaderCache(t *testing.T) {
	// The banners the header scheme cached as img/gameicons/<appid>.jpg would
	// otherwise stay in wwwroot forever, unread.
	square := filepath.Join(t.TempDir(), "wwwroot", "img", "gameicons", "square")
	legacy := filepath.Dir(square)
	writeIconFile(t, filepath.Join(legacy, "730.jpg"), jpegBytes())
	writeIconFile(t, filepath.Join(legacy, "440.jpg"), jpegBytes())
	writeIconFile(t, filepath.Join(legacy, "notes.txt"), []byte("keep me"))
	writeIconFile(t, filepath.Join(legacy, "placeholder.jpg"), jpegBytes())
	writeIconFile(t, filepath.Join(square, "730.jpg"), jpegBytes())

	pruneLegacyGameIcons(square)

	for _, name := range []string{"730.jpg", "440.jpg"} {
		if _, err := os.Stat(filepath.Join(legacy, name)); err == nil {
			t.Fatalf("legacy banner %s survived the prune", name)
		}
	}
	for _, path := range []string{filepath.Join(legacy, "notes.txt"), filepath.Join(legacy, "placeholder.jpg"), filepath.Join(square, "730.jpg")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("prune removed %s", path)
		}
	}
}

func TestLooksLikeJPEG(t *testing.T) {
	if !looksLikeJPEG(jpegBytes()) {
		t.Fatal("jpeg magic rejected")
	}
	for _, b := range [][]byte{
		nil,
		[]byte("<!DOCTYPE html><html>404</html>"),
		{0x89, 'P', 'N', 'G', 0x0D},
		{0xFF, 0xD8}, // truncated to less than the magic
	} {
		if looksLikeJPEG(b) {
			t.Fatalf("accepted non-jpeg payload %q", b)
		}
	}
}

func TestWarmGameIconsUsesCacheAndSkipsInvalid(t *testing.T) {
	dir, doer := useTempIconCache(t)
	for _, id := range []string{"730", "440"} {
		writeIconFile(t, filepath.Join(dir, id+".jpg"), jpegBytes())
	}

	got := WarmGameIcons(context.Background(), []string{"730", "440", "730", "", "../evil", "0"})
	if len(got) != 2 {
		t.Fatalf("got %d urls: %v", len(got), got)
	}
	if got["730"] != "/img/gameicons/square/730.jpg" || got["440"] != "/img/gameicons/square/440.jpg" {
		t.Fatalf("unexpected urls: %v", got)
	}
	if n := doer.calls.Load(); n != 0 {
		t.Fatalf("warm made %d HTTP requests for cached ids", n)
	}
}

func TestWarmGameIconsStopsOnCancelledContext(t *testing.T) {
	_, doer := useTempIconCache(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ids := make([]string, 0, 200)
	for i := 0; i < 200; i++ {
		ids = append(ids, "10000"+string(rune('0'+i%10)))
	}
	if got := WarmGameIcons(ctx, ids); len(got) != 0 {
		t.Fatalf("cancelled warm resolved %d urls", len(got))
	}
	if n := doer.calls.Load(); n != 0 {
		t.Fatalf("cancelled warm made %d HTTP requests", n)
	}
}
