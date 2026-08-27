// Package gzipfs presents a partially gzip-compressed asset tree as if nothing
// were compressed: a file stored as "x.js.gz" opens, stats and lists under the
// name "x.js" with its inflated size, and inflates as it is read. Consumers —
// the Wails asset server, the embedded platform art lookup — need no knowledge
// of the compression, and files that grow under gzip are simply stored raw and
// pass straight through.
package gzipfs

import (
	"compress/gzip"
	"encoding/binary"
	"io"
	"io/fs"
	"path"
	"strings"
	"sync"
	"time"
)

const suffix = ".gz"

// FS wraps an fs.FS whose files may be stored as <name>.gz.
type FS struct{ inner fs.FS }

var _ interface {
	fs.FS
	fs.StatFS
	fs.ReadDirFS
} = (*FS)(nil)

func New(inner fs.FS) *FS { return &FS{inner: inner} }

func (f *FS) Open(name string) (fs.File, error) {
	file, err := f.inner.Open(name)
	if err != nil {
		gz, gzErr := f.inner.Open(name + suffix)
		if gzErr != nil {
			// The caller asked for `name`; report the miss under that name.
			return nil, err
		}
		return newGzipFile(gz, path.Base(name), func() (fs.File, error) {
			return f.inner.Open(name + suffix)
		})
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if info.IsDir() {
		return &dirFile{File: file, dir: name, fsys: f}, nil
	}
	return file, nil
}

func (f *FS) Stat(name string) (fs.FileInfo, error) {
	info, err := fs.Stat(f.inner, name)
	if err == nil {
		return info, nil
	}
	gzInfo, gzErr := f.statGzip(name+suffix, path.Base(name))
	if gzErr != nil {
		return nil, err
	}
	return gzInfo, nil
}

func (f *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(f.inner, name)
	if err != nil {
		return nil, err
	}
	return f.present(name, entries), nil
}

func (f *FS) present(dir string, entries []fs.DirEntry) []fs.DirEntry {
	for i, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		entries[i] = gzipEntry{
			name: strings.TrimSuffix(e.Name(), suffix),
			full: path.Join(dir, e.Name()),
			fsys: f,
		}
	}
	return entries
}

func (f *FS) statGzip(full, name string) (fs.FileInfo, error) {
	file, err := f.inner.Open(full)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return gzipInfo(file, name)
}

type dirFile struct {
	fs.File
	dir  string
	fsys *FS
}

func (d *dirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	rd, ok := d.File.(fs.ReadDirFile)
	if !ok {
		return nil, &fs.PathError{Op: "readdir", Path: d.dir, Err: fs.ErrInvalid}
	}
	entries, err := rd.ReadDir(n)
	return d.fsys.present(d.dir, entries), err
}

type gzipEntry struct {
	name string
	full string
	fsys *FS
}

func (e gzipEntry) Name() string               { return e.name }
func (e gzipEntry) IsDir() bool                { return false }
func (e gzipEntry) Type() fs.FileMode          { return 0 }
func (e gzipEntry) Info() (fs.FileInfo, error) { return e.fsys.statGzip(e.full, e.name) }

// gzipFile inflates on demand. It deliberately does not implement io.Seeker:
// Wails' asset file server falls back to a Content-Length write for non-seekers,
// which is the cheap path here, and a seekable stream would mean buffering the
// whole inflated file.
type gzipFile struct {
	raw  fs.File
	info fs.FileInfo
	zr   *gzip.Reader
	// Set only when raw cannot seek: sizing consumed the member, so the first
	// read has to start over from a fresh handle.
	reopen func() (fs.File, error)
}

func newGzipFile(raw fs.File, name string, reopen func() (fs.File, error)) (fs.File, error) {
	info, err := gzipInfo(raw, name)
	if err != nil {
		raw.Close()
		return nil, err
	}
	f := &gzipFile{raw: raw, info: info}
	if _, ok := raw.(io.Seeker); !ok {
		f.reopen = reopen
	}
	return f, nil
}

func (f *gzipFile) Stat() (fs.FileInfo, error) { return f.info, nil }

func (f *gzipFile) Read(p []byte) (int, error) {
	if f.zr == nil {
		if s, ok := f.raw.(io.Seeker); ok {
			if _, err := s.Seek(0, io.SeekStart); err != nil {
				return 0, err
			}
		} else if f.reopen != nil {
			raw, err := f.reopen()
			if err != nil {
				return 0, err
			}
			f.raw.Close()
			f.raw = raw
			f.reopen = nil
		}
		zr := readerPool.Get().(*gzip.Reader)
		if err := zr.Reset(f.raw); err != nil {
			readerPool.Put(zr)
			return 0, err
		}
		f.zr = zr
	}
	return f.zr.Read(p)
}

func (f *gzipFile) Close() error {
	if f.zr != nil {
		readerPool.Put(f.zr)
		f.zr = nil
	}
	return f.raw.Close()
}

var readerPool = sync.Pool{New: func() any { return new(gzip.Reader) }}

type fileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
}

func (i fileInfo) Name() string       { return i.name }
func (i fileInfo) Size() int64        { return i.size }
func (i fileInfo) Mode() fs.FileMode  { return i.mode }
func (i fileInfo) ModTime() time.Time { return i.modTime }
func (i fileInfo) IsDir() bool        { return false }
func (i fileInfo) Sys() any           { return nil }

// gzipInfo reports the member's inflated size from its ISIZE trailer rather than
// inflating it. ISIZE is mod 2^32, which is exact here because the build writes
// these members and no asset approaches 4 GiB.
func gzipInfo(raw fs.File, name string) (fs.FileInfo, error) {
	rawInfo, err := raw.Stat()
	if err != nil {
		return nil, err
	}
	size, err := inflatedSize(raw, rawInfo.Size())
	if err != nil {
		return nil, err
	}
	return fileInfo{name: name, size: size, mode: rawInfo.Mode(), modTime: rawInfo.ModTime()}, nil
}

func inflatedSize(raw fs.File, compressed int64) (int64, error) {
	if s, ok := raw.(io.Seeker); ok && compressed >= 4 {
		if _, err := s.Seek(-4, io.SeekEnd); err == nil {
			var b [4]byte
			if _, err := io.ReadFull(raw, b[:]); err == nil {
				return int64(binary.LittleEndian.Uint32(b[:])), nil
			}
		}
		if _, err := s.Seek(0, io.SeekStart); err != nil {
			return 0, err
		}
	}
	zr, err := gzip.NewReader(raw)
	if err != nil {
		return 0, err
	}
	defer zr.Close()
	return io.Copy(io.Discard, zr)
}
