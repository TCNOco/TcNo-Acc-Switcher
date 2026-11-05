package authenticatorstorage

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"TcNo-Acc-Switcher/internal/steamguard/securefile"
)

const (
	started   = ".authenticator-migration-started"
	completed = ".authenticator-migration-complete"
)

var migrationMu sync.Mutex
var resolved = map[string]bool{}

func ValidatePath(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("authenticator storage path must be absolute")
	}
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		linked, err := securefile.IsReparsePoint(current)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if linked {
			return errors.New("authenticator storage path contains a link")
		}
		if filepath.Dir(current) == current {
			return nil
		}
	}
}

func Directory(root, legacy, name string) (string, error) {
	if !(legacy == "SteamGuard" && name == "Authenticators" || legacy == "SteamBrowser" && name == "AccountBrowser") {
		return "", errors.New("unknown authenticator directory")
	}
	migrationMu.Lock()
	defer migrationMu.Unlock()
	source, target := filepath.Join(root, legacy), filepath.Join(root, name)
	witness := filepath.Join(root, ".authenticator-storage-"+name)
	if err := ValidatePath(witness); err != nil {
		return "", err
	}
	choice, choiceErr := os.ReadFile(witness)
	if choiceErr == nil && bytes.Equal(choice, []byte("complete\n")) {
		resolved[target] = true
		if err := ValidatePath(target); err != nil {
			return "", err
		}
		return target, nil
	}
	if choiceErr != nil && !os.IsNotExist(choiceErr) {
		return "", choiceErr
	}
	if resolved[target] {
		if err := ValidatePath(target); err != nil {
			return "", err
		}
		return target, nil
	}
	for _, path := range []string{source, target} {
		if err := ValidatePath(path); err != nil {
			return "", err
		}
	}
	old, err := os.Stat(source)
	if os.IsNotExist(err) {
		resolved[target] = true
		return target, nil
	}
	if err != nil {
		return "", err
	}
	if !old.IsDir() {
		return "", errors.New("legacy authenticator path is not a directory")
	}
	entries, err := os.ReadDir(target)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err == nil {
		for _, name := range []string{started, completed} {
			if err := ValidatePath(filepath.Join(target, name)); err != nil {
				return "", err
			}
		}
		if ready, err := os.ReadFile(filepath.Join(target, completed)); err == nil && bytes.Equal(ready, []byte("complete\n")) {
			if err := writeMarker(witness, []byte("complete\n")); err != nil {
				return "", err
			}
			resolved[target] = true
			return target, nil
		}
		marker, markErr := os.ReadFile(filepath.Join(target, started))
		if os.IsNotExist(markErr) && len(entries) > 0 {
			if err := writeMarker(witness, []byte("complete\n")); err != nil {
				return "", err
			}
			resolved[target] = true
			return target, nil
		}
		if markErr != nil && !os.IsNotExist(markErr) {
			return "", markErr
		}
		if markErr == nil && !bytes.Equal(marker, []byte("copying\n")) {
			return "", errors.New("invalid authenticator migration marker")
		}
	} else if err := securefile.CreateDirectoryNew(target); err != nil {
		return "", err
	}
	if err := writeMarker(filepath.Join(target, started), []byte("copying\n")); err != nil {
		return "", err
	}
	manifest := map[string][32]byte{}
	var total int64
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == started || rel == completed {
			return nil
		}
		if err := ValidatePath(path); err != nil {
			return err
		}
		destination := filepath.Join(target, rel)
		if err := ValidatePath(destination); err != nil {
			return err
		}
		if entry.IsDir() {
			if rel == "." {
				return nil
			}
			if _, err := os.Stat(destination); os.IsNotExist(err) {
				return securefile.CreateDirectoryNew(destination)
			} else {
				return err
			}
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || len(manifest) >= 100000 || info.Size() < 0 || total+info.Size() > 8<<30 {
			return errors.New("authenticator migration exceeds supported file types or size")
		}
		total += info.Size()
		digest, err := copyFile(path, destination, info)
		if err != nil {
			return err
		}
		manifest[rel] = digest
		return nil
	})
	if err != nil {
		return "", err
	}
	for rel, expected := range manifest {
		for _, path := range []string{filepath.Join(source, rel), filepath.Join(target, rel)} {
			if err := ValidatePath(path); err != nil {
				return "", err
			}
			actual, err := fileHash(path)
			if err != nil {
				return "", err
			}
			if actual != expected {
				return "", errors.New("authenticator data changed during migration; close the older application and retry")
			}
		}
	}
	if err := writeMarker(filepath.Join(target, completed), []byte("complete\n")); err != nil {
		return "", err
	}
	if err := writeMarker(witness, []byte("complete\n")); err != nil {
		return "", err
	}
	resolved[target] = true
	return target, nil
}

func copyFile(source, target string, expected os.FileInfo) ([32]byte, error) {
	var zero [32]byte
	in, err := os.Open(source)
	if err != nil {
		return zero, err
	}
	defer in.Close()
	actual, err := in.Stat()
	if err != nil {
		return zero, err
	}
	if !os.SameFile(expected, actual) || !actual.Mode().IsRegular() {
		return zero, errors.New("migration source changed")
	}
	out, err := securefile.CreateNew(target)
	if os.IsExist(err) {
		out, err = os.OpenFile(target, os.O_WRONLY|os.O_TRUNC, 0o600)
	}
	if err != nil {
		return zero, err
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(out, hash), io.LimitReader(in, expected.Size()+1))
	syncErr, closeErr := out.Sync(), out.Close()
	if err := errors.Join(copyErr, syncErr, closeErr); err != nil {
		return zero, err
	}
	if written != expected.Size() {
		return zero, errors.New("migration source changed size")
	}
	copy(zero[:], hash.Sum(nil))
	return zero, nil
}

func fileHash(path string) ([32]byte, error) {
	var result [32]byte
	info, err := os.Lstat(path)
	if err != nil {
		return result, err
	}
	if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > 8<<30 {
		return result, errors.New("unsupported migration file")
	}
	f, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer f.Close()
	actual, err := f.Stat()
	if err != nil {
		return result, err
	}
	if !os.SameFile(info, actual) {
		return result, errors.New("migration file changed")
	}
	hash := sha256.New()
	if size, err := io.Copy(hash, io.LimitReader(f, info.Size()+1)); err != nil {
		return result, err
	} else if size != info.Size() {
		return result, errors.New("migration file changed size")
	}
	copy(result[:], hash.Sum(nil))
	return result, nil
}

func writeMarker(path string, data []byte) error {
	if err := ValidatePath(path); err != nil {
		return err
	}
	f, err := securefile.CreateNew(path)
	if os.IsExist(err) {
		f, err = os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o600)
	}
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	return errors.Join(writeErr, f.Sync(), f.Close())
}

func Settings(root string) (string, error) {
	migrationMu.Lock()
	defer migrationMu.Unlock()
	target := filepath.Join(root, "Settings", "Authenticators.json")
	if err := ValidatePath(target); err != nil {
		return "", err
	}
	readyMarker := filepath.Join(root, "Settings", ".authenticator-settings-migration-complete")
	if err := ValidatePath(readyMarker); err != nil {
		return "", err
	}
	if ready, err := os.ReadFile(readyMarker); err == nil && bytes.Equal(ready, []byte("complete\n")) {
		return target, nil
	}
	if _, err := os.Stat(target); err == nil {
		for _, name := range []string{".authenticator-settings-migration-started", ".authenticator-settings-migration-complete"} {
			if err := ValidatePath(filepath.Join(root, "Settings", name)); err != nil {
				return "", err
			}
		}
		marker, markerErr := os.ReadFile(filepath.Join(root, "Settings", ".authenticator-settings-migration-started"))
		if os.IsNotExist(markerErr) {
			return target, nil
		}
		if markerErr != nil {
			return "", markerErr
		}
		if !bytes.Equal(marker, []byte("copying\n")) {
			return "", errors.New("invalid authenticator settings migration marker")
		}
		ready, readyErr := os.ReadFile(filepath.Join(root, "Settings", ".authenticator-settings-migration-complete"))
		if readyErr == nil && bytes.Equal(ready, []byte("complete\n")) {
			return target, nil
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	source := filepath.Join(root, "Settings", "SteamGuard.json")
	if err := ValidatePath(source); err != nil {
		return "", err
	}
	info, err := os.Stat(source)
	if os.IsNotExist(err) {
		return target, nil
	}
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > 65536 {
		return "", errors.New("invalid legacy authenticator settings")
	}
	if err := writeMarker(filepath.Join(root, "Settings", ".authenticator-settings-migration-started"), []byte("copying\n")); err != nil {
		return "", err
	}
	expected, err := copyFile(source, target, info)
	if err != nil {
		return "", err
	}
	actual, err := fileHash(target)
	if err != nil {
		return "", err
	}
	if actual != expected {
		return "", errors.New("authenticator settings migration verification failed")
	}
	current, err := fileHash(source)
	if err != nil {
		return "", err
	}
	if current != expected {
		return "", errors.New("legacy authenticator settings changed during migration")
	}
	if err := writeMarker(filepath.Join(root, "Settings", ".authenticator-settings-migration-complete"), []byte("complete\n")); err != nil {
		return "", err
	}
	return target, nil
}
