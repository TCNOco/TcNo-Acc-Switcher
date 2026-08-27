package gzipfs

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"testing"
	"testing/fstest"
)

// grew stands in for the files that gzip makes bigger (webp, png, woff2): the
// build leaves those uncompressed and they must pass straight through.
const grew = "img/tile.webp"

func newTestFS(t *testing.T) (*FS, []byte) {
	t.Helper()
	// Repetitive so the compressed member is genuinely smaller, and long enough
	// that reads span several buffers.
	plain := bytes.Repeat([]byte("<!doctype html><script src=/assets/app.js>"), 1000)
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if _, err := zw.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return New(fstest.MapFS{
		"index.html.gz": {Data: buf.Bytes()},
		grew:            {Data: []byte("\x00webp bytes")},
	}), plain
}

func TestOpenInflates(t *testing.T) {
	fsys, plain := newTestFS(t)

	got, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("inflated %d bytes, want %d", len(got), len(plain))
	}

	f, err := fsys.Open("index.html")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()
	if _, ok := f.(io.Seeker); ok {
		t.Fatal("compressed file must not be an io.Seeker: Wails would try to ServeContent it")
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	// Content-Length comes from here; a mismatch truncates or hangs WebView2.
	if info.Size() != int64(len(plain)) {
		t.Fatalf("Stat size %d, want inflated %d", info.Size(), len(plain))
	}
	if info.Name() != "index.html" {
		t.Fatalf("Stat name %q", info.Name())
	}
}

func TestStatSizeMatchesUncompressedFile(t *testing.T) {
	fsys, _ := newTestFS(t)
	info, err := fs.Stat(fsys, grew)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() != int64(len("\x00webp bytes")) {
		t.Fatalf("Stat size %d", info.Size())
	}
	got, err := fs.ReadFile(fsys, grew)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "\x00webp bytes" {
		t.Fatalf("read %q", got)
	}
}

func TestReadDirHidesSuffix(t *testing.T) {
	fsys, plain := newTestFS(t)
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
		if e.Name() == "index.html" {
			info, err := e.Info()
			if err != nil {
				t.Fatalf("Info: %v", err)
			}
			if info.Size() != int64(len(plain)) {
				t.Fatalf("DirEntry size %d, want %d", info.Size(), len(plain))
			}
		}
	}
	want := []string{"img", "index.html"}
	if len(names) != len(want) {
		t.Fatalf("entries %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("entries %v, want %v", names, want)
		}
	}
}

// The Wails asset server 404s only on fs.ErrNotExist, so a miss must not
// surface as some other error.
func TestMissingFile(t *testing.T) {
	fsys, _ := newTestFS(t)
	if _, err := fsys.Open("nope.js"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Open err %v", err)
	}
	if _, err := fs.Stat(fsys, "nope.js"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Stat err %v", err)
	}
}

// ISIZE is read by seeking to the member's tail. An fs.FS whose files cannot
// seek has to fall back to inflating, and must report the same size.
func TestSizeWithoutSeeker(t *testing.T) {
	fsys, plain := newTestFS(t)
	noSeek := New(unseekableFS{fsys.inner})
	info, err := fs.Stat(noSeek, "index.html")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() != int64(len(plain)) {
		t.Fatalf("Stat size %d, want %d", info.Size(), len(plain))
	}
	got, err := fs.ReadFile(noSeek, "index.html")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("inflated %d bytes, want %d", len(got), len(plain))
	}
}

type unseekableFS struct{ fs.FS }

func (u unseekableFS) Open(name string) (fs.File, error) {
	f, err := u.FS.Open(name)
	if err != nil {
		return nil, err
	}
	return unseekableFile{f}, nil
}

type unseekableFile struct{ fs.File }
