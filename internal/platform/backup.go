package platform

import (
	"archive/zip"
	"compress/flate"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"TcNo-Acc-Switcher/internal/fsutil"
)

type BackupResult struct {
	ArchivePath  string `json:"archivePath"`
	Files        int    `json:"files"`
	Bytes        int64  `json:"bytes"`
	RestoredFrom string `json:"restoredFrom,omitempty"`
}

type backupMapping struct {
	sourcePath string
	archiveRel string
}

func (p *PlatformService) HasPlatformBackupFolders(platformKey string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	d, err := p.loadDescriptorUnlocked(platformKey)
	if err != nil {
		return false, err
	}
	for src := range d.Extras.BackupFolders {
		if strings.TrimSpace(src) != "" {
			return true, nil
		}
	}
	return false, nil
}

func (p *PlatformService) BackupPlatform(platformKey string, everything bool) (BackupResult, error) {
	p.mu.Lock()
	d, err := p.loadDescriptorUnlocked(platformKey)
	if err != nil {
		p.mu.Unlock()
		return BackupResult{}, err
	}
	folder, err := p.getPlatformInstallFolderUnlocked(platformKey)
	p.mu.Unlock()
	if err != nil {
		return BackupResult{}, err
	}

	mappings, err := resolveBackupMappings(d, PathTokenContext{PlatformFolder: folder})
	if err != nil {
		return BackupResult{}, err
	}
	if len(mappings) == 0 {
		return BackupResult{}, errors.New("no backup folders configured")
	}

	backupDir, err := platformBackupDir(platformKey)
	if err != nil {
		return BackupResult{}, err
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return BackupResult{}, err
	}
	archive := filepath.Join(
		backupDir,
		fmt.Sprintf("Backup_%s_%s.zip", safeBackupPlatformName(platformKey), time.Now().Format("2006-01-02_15-04-05")),
	)
	files, bytesWritten, err := writeBackupArchive(archive, mappings, d.Extras.BackupFileTypesInclude, d.Extras.BackupFileTypesIgnore, everything)
	if err != nil {
		return BackupResult{}, err
	}
	return BackupResult{
		ArchivePath: archive,
		Files:       files,
		Bytes:       bytesWritten,
	}, nil
}

func (p *PlatformService) OpenPlatformBackupFolder(platformKey string) error {
	backupDir, err := platformBackupDir(platformKey)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return err
	}
	return OpenPathInFileManager(backupDir)
}

func (p *PlatformService) RestoreLatestPlatformBackup(platformKey string) (BackupResult, error) {
	p.mu.Lock()
	d, err := p.loadDescriptorUnlocked(platformKey)
	if err != nil {
		p.mu.Unlock()
		return BackupResult{}, err
	}
	folder, err := p.getPlatformInstallFolderUnlocked(platformKey)
	p.mu.Unlock()
	if err != nil {
		return BackupResult{}, err
	}

	mappings, err := resolveBackupMappings(d, PathTokenContext{PlatformFolder: folder})
	if err != nil {
		return BackupResult{}, err
	}
	if len(mappings) == 0 {
		return BackupResult{}, errors.New("no backup folders configured")
	}

	backupDir, err := platformBackupDir(platformKey)
	if err != nil {
		return BackupResult{}, err
	}
	archive, err := latestBackupZip(backupDir, safeBackupPlatformName(platformKey))
	if err != nil {
		return BackupResult{}, err
	}

	restoreRoot, err := os.MkdirTemp(backupDir, "restore-*")
	if err != nil {
		return BackupResult{}, err
	}
	defer func() { _ = os.RemoveAll(restoreRoot) }()

	files, bytesWritten, err := extractZipSafely(archive, restoreRoot)
	if err != nil {
		return BackupResult{}, err
	}
	if err := restoreExtractedBackup(restoreRoot, mappings); err != nil {
		return BackupResult{}, err
	}

	return BackupResult{
		ArchivePath:  archive,
		Files:        files,
		Bytes:        bytesWritten,
		RestoredFrom: archive,
	}, nil
}

func resolveBackupMappings(d Descriptor, ctx PathTokenContext) ([]backupMapping, error) {
	if len(d.Extras.BackupFolders) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(d.Extras.BackupFolders))
	for src := range d.Extras.BackupFolders {
		keys = append(keys, src)
	}
	sort.Strings(keys)

	out := make([]backupMapping, 0, len(keys))
	for _, src := range keys {
		sourcePath, err := resolveBackupSourcePath(src, ctx)
		if err != nil {
			return nil, err
		}
		archiveRel, err := sanitizeBackupRelativePath(d.Extras.BackupFolders[src], sourcePath)
		if err != nil {
			return nil, err
		}
		out = append(out, backupMapping{sourcePath: sourcePath, archiveRel: archiveRel})
	}
	return out, nil
}

func resolveBackupSourcePath(raw string, ctx PathTokenContext) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty backup source path")
	}
	expanded := ExpandPathTokens(raw, ctx)
	if hasPlaceholderToken(expanded) {
		return "", fmt.Errorf("backup source path has unresolved placeholder: %s", raw)
	}
	expanded = filepath.Clean(strings.TrimSpace(expanded))
	if expanded == "" || expanded == "." || isVolumeRoot(expanded) {
		return "", fmt.Errorf("unsafe backup source path resolves to root: %s", raw)
	}
	return expanded, nil
}

func sanitizeBackupRelativePath(rel, sourcePath string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		rel = filepath.Base(sourcePath)
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("backup relative path must not be absolute: %s", rel)
	}
	rel = filepath.Clean(filepath.FromSlash(rel))
	if rel == "." || strings.HasPrefix(rel, "..") || strings.Contains(rel, ":") {
		return "", fmt.Errorf("unsafe backup relative path: %s", rel)
	}
	return rel, nil
}

func shouldIncludeBackupFile(path string, includeSet, ignoreSet map[string]struct{}, everything bool) bool {
	if everything {
		return true
	}
	ext := strings.ToLower(filepath.Ext(path))
	if len(includeSet) > 0 {
		_, ok := includeSet[ext]
		return ok
	}
	if len(ignoreSet) > 0 {
		_, skip := ignoreSet[ext]
		return !skip
	}
	return true
}

func normalizeExtensionSet(exts []string) map[string]struct{} {
	out := make(map[string]struct{}, len(exts))
	for _, ext := range exts {
		ext = strings.TrimSpace(strings.ToLower(ext))
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		out[ext] = struct{}{}
	}
	return out
}

func writeBackupArchive(archivePath string, mappings []backupMapping, include, ignore []string, everything bool) (int, int64, error) {
	f, err := os.Create(archivePath)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = f.Close() }()

	zw := zip.NewWriter(f)
	zw.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) {
		return newDeflateBestWriter(w)
	})

	includeSet := normalizeExtensionSet(include)
	ignoreSet := normalizeExtensionSet(ignore)
	files := 0
	var bytesWritten int64

	for _, m := range mappings {
		st, err := os.Stat(m.sourcePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			_ = zw.Close()
			return 0, 0, err
		}

		if !st.IsDir() {
			if !shouldIncludeBackupFile(m.sourcePath, includeSet, ignoreSet, everything) {
				continue
			}
			n, err := addFileToZip(zw, m.sourcePath, m.archiveRel)
			if err != nil {
				_ = zw.Close()
				return 0, 0, err
			}
			files++
			bytesWritten += n
			continue
		}

		err = filepath.WalkDir(m.sourcePath, func(path string, de os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if de.IsDir() {
				return nil
			}
			if !shouldIncludeBackupFile(path, includeSet, ignoreSet, everything) {
				return nil
			}
			rel, err := filepath.Rel(m.sourcePath, path)
			if err != nil {
				return err
			}
			zipPath := filepath.Join(m.archiveRel, rel)
			n, err := addFileToZip(zw, path, zipPath)
			if err != nil {
				return err
			}
			files++
			bytesWritten += n
			return nil
		})
		if err != nil {
			_ = zw.Close()
			return 0, 0, err
		}
	}

	if err := zw.Close(); err != nil {
		return 0, 0, err
	}
	if err := f.Sync(); err != nil {
		return 0, 0, err
	}
	return files, bytesWritten, nil
}

func newDeflateBestWriter(w io.Writer) (io.WriteCloser, error) {
	return flate.NewWriter(w, flate.BestCompression)
}

func addFileToZip(zw *zip.Writer, srcPath, archivePath string) (int64, error) {
	archivePath = filepath.ToSlash(filepath.Clean(archivePath))
	if strings.HasPrefix(archivePath, "/") || strings.HasPrefix(archivePath, "../") || archivePath == "." {
		return 0, fmt.Errorf("unsafe archive entry path: %s", archivePath)
	}
	info, err := os.Stat(srcPath)
	if err != nil {
		return 0, err
	}
	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		return 0, err
	}
	hdr.Name = archivePath
	hdr.Method = zip.Deflate
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return 0, err
	}
	in, err := os.Open(srcPath)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	n, err := io.Copy(w, in)
	return n, err
}

func latestBackupZip(backupDir, safePlatform string) (string, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return "", err
	}
	prefix := "Backup_" + safePlatform + "_"
	var newest string
	var newestTime time.Time
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".zip") {
			continue
		}
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		fi, err := entry.Info()
		if err != nil {
			continue
		}
		if newest == "" || fi.ModTime().After(newestTime) {
			newestTime = fi.ModTime()
			newest = filepath.Join(backupDir, name)
		}
	}
	if newest == "" {
		return "", errors.New("no backup archive found")
	}
	return newest, nil
}

func extractZipSafely(zipPath, destDir string) (int, int64, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0, 0, err
	}
	defer zr.Close()

	files := 0
	var bytesWritten int64
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		outPath, err := safeJoinZipPath(destDir, f.Name)
		if err != nil {
			return 0, 0, err
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return 0, 0, err
		}
		rc, err := f.Open()
		if err != nil {
			return 0, 0, err
		}
		out, err := os.Create(outPath)
		if err != nil {
			_ = rc.Close()
			return 0, 0, err
		}
		n, err := io.Copy(out, rc)
		_ = out.Close()
		_ = rc.Close()
		if err != nil {
			return 0, 0, err
		}
		files++
		bytesWritten += n
	}
	return files, bytesWritten, nil
}

func safeJoinZipPath(baseDir, archiveName string) (string, error) {
	archiveName = filepath.FromSlash(strings.TrimSpace(archiveName))
	if archiveName == "" {
		return "", errors.New("empty archive entry")
	}
	clean := filepath.Clean(archiveName)
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") || strings.Contains(clean, ":") {
		return "", fmt.Errorf("unsafe archive entry: %s", archiveName)
	}
	target := filepath.Join(baseDir, clean)
	baseClean := filepath.Clean(baseDir) + string(filepath.Separator)
	targetClean := filepath.Clean(target)
	if targetClean != filepath.Clean(baseDir) && !strings.HasPrefix(targetClean, baseClean) {
		return "", fmt.Errorf("unsafe archive entry path: %s", archiveName)
	}
	return targetClean, nil
}

func restoreExtractedBackup(restoreRoot string, mappings []backupMapping) error {
	for _, m := range mappings {
		from := filepath.Join(restoreRoot, filepath.FromSlash(m.archiveRel))
		st, err := os.Stat(from)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if err := ValidateDeleteTargetPath(m.sourcePath); err != nil {
			return err
		}
		if st.IsDir() {
			if err := fsutil.CopyDir(from, m.sourcePath); err != nil {
				return err
			}
			continue
		}
		in, err := os.Open(from)
		if err != nil {
			return err
		}
		data, err := io.ReadAll(in)
		_ = in.Close()
		if err != nil {
			return err
		}
		if err := fsutil.WriteFileAtomic(m.sourcePath, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func platformBackupDir(platformKey string) (string, error) {
	ud, err := EffectiveUserDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(ud, "Backups", safeBackupPlatformName(platformKey)), nil
}

func safeBackupPlatformName(platformKey string) string {
	return sanitizePlatformSettingsFilePrefix(platformKey)
}
