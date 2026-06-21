package main

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type FilesystemService struct{}

type FsDirEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
}

type PathStat struct {
	Exists bool `json:"exists"`
	IsDir  bool `json:"isDir"`
}

func (f *FilesystemService) StatPath(raw string) PathStat {
	p := strings.TrimSpace(raw)
	if p == "" {
		return PathStat{}
	}
	if runtime.GOOS == "windows" {
		vol := filepath.VolumeName(p)
		if vol != "" && len(p) == len(vol) {
			p = vol + `\`
		}
	}
	p = filepath.Clean(p)
	if runtime.GOOS == "windows" && filepath.VolumeName(p) != "" {
		v := filepath.VolumeName(p)
		rest := strings.TrimPrefix(p, v)
		if rest == "" || rest == "." {
			p = v + `\`
		}
	}
	fi, err := os.Stat(p)
	if err != nil {
		return PathStat{}
	}
	return PathStat{Exists: true, IsDir: fi.IsDir()}
}

func (f *FilesystemService) ListRoots() ([]string, error) {
	if runtime.GOOS == "windows" {
		var roots []string
		for _, r := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
			p := string(r) + `:\`
			fi, err := os.Stat(p)
			if err != nil || !fi.IsDir() {
				continue
			}

			v := string(r) + ":"
			roots = append(roots, v+`\`)
		}
		return roots, nil
	}
	return []string{"/"}, nil
}

func (f *FilesystemService) ListDir(dirPath string) ([]FsDirEntry, error) {
	dirPath = strings.TrimSpace(dirPath)
	if dirPath == "" {
		return nil, os.ErrInvalid
	}
	if runtime.GOOS == "windows" {
		vol := filepath.VolumeName(dirPath)
		if vol != "" && len(dirPath) == len(vol) {
			dirPath = vol + `\`
		}
	}
	dirPath = filepath.Clean(dirPath)
	if runtime.GOOS == "windows" && filepath.VolumeName(dirPath) != "" {
		v := filepath.VolumeName(dirPath)
		rest := strings.TrimPrefix(dirPath, v)
		if rest == "" || rest == "." {
			dirPath = v + `\`
		}
	}

	ents, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	out := make([]FsDirEntry, 0, len(ents))
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(dirPath, name)
		fi, err := os.Stat(full)
		if err != nil {
			continue
		}
		full = filepath.Clean(full)
		out = append(out, FsDirEntry{Name: name, Path: full, IsDir: fi.IsDir()})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}
