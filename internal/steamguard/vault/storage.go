package vault

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

const (
	headerName  = "header.json"
	keyringName = "keyring.bin"
	activeName  = "active"
	journalName = "transaction.json"
	recordsName = "records"
	maxHeader   = 64 << 10
	maxKeyring  = 24 << 20
	maxRecord   = 40 << 20
)

var idPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type header struct {
	Version      int              `json:"version"`
	VaultID      string           `json:"vaultId"`
	KeyringID    string           `json:"keyringId"`
	KDF          KDFParams        `json:"kdf"`
	Salt         string           `json:"salt"`
	VaultKey     envelope         `json:"vaultKey"`
	OuterVersion int              `json:"outerVersion,omitempty"`
	OuterProof   envelope         `json:"outerProof,omitempty"`
	Recovery     *recoveryWrapper `json:"recovery,omitempty"`
}

type recoveryWrapper struct {
	Version  int       `json:"version"`
	KDF      KDFParams `json:"kdf"`
	Salt     string    `json:"salt"`
	OuterKey envelope  `json:"outerKey"`
}

type keyringPayload struct {
	Version int           `json:"version"`
	Records []recordEntry `json:"records"`
}

type outerFile struct {
	Version    int      `json:"version"`
	Ciphertext envelope `json:"ciphertext"`
}

type recordEntry struct {
	ID         string   `json:"id"`
	SteamID64  string   `json:"steamId64"`
	Filename   string   `json:"filename"`
	WrappedKey envelope `json:"wrappedKey"`
}

type recordFile struct {
	Version    int      `json:"version"`
	RecordID   string   `json:"recordId"`
	Ciphertext envelope `json:"ciphertext"`
}

type transaction struct {
	Version  int    `json:"version"`
	Previous string `json:"previous,omitempty"`
	Next     string `json:"next"`
}

func readJSONFile(path string, max int, dst any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	limited := io.LimitReader(f, int64(max)+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if len(data) > max {
		return ErrInvalidFormat
	}
	return unmarshalStrict(data, dst)
}

func unmarshalStrict(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return errors.Join(ErrInvalidFormat, err)
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return ErrInvalidFormat
	}
	return nil
}

func marshalJSON(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func atomicWrite(path string, data []byte, perm os.FileMode, hardener Hardener) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".write-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := hardener.HardenFile(tmpPath); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	ok = true
	if err := hardener.HardenFile(path); err != nil {
		return err
	}
	syncDir(dir)
	return nil
}

func syncDir(path string) {
	f, err := os.Open(path)
	if err == nil {
		_ = f.Sync()
		_ = f.Close()
	}
}

func validID(s string) bool { return idPattern.MatchString(s) }

func generationPath(root, id string) (string, error) {
	if !validID(id) {
		return "", ErrInvalidFormat
	}
	return filepath.Join(root, "generation-"+id), nil
}

func recordPath(genPath, name string) (string, error) {
	if !validID(name) {
		return "", ErrInvalidFormat
	}
	return filepath.Join(genPath, recordsName, name+".bin"), nil
}

func readActive(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, activeName))
	if err != nil {
		return "", err
	}
	if len(b) != 36 || !validID(string(b)) {
		return "", ErrInvalidFormat
	}
	return string(b), nil
}

func recoverTransaction(root string) error {
	path := filepath.Join(root, journalName)
	var tx transaction
	if err := readJSONFile(path, maxHeader, &tx); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if tx.Version != FormatVersion || !validID(tx.Next) ||
		(tx.Previous != "" && (!validID(tx.Previous) || tx.Previous == tx.Next)) {
		return ErrInvalidFormat
	}
	active, err := readActive(root)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if active != tx.Next {
		gen, err := generationPath(root, tx.Next)
		if err != nil {
			return err
		}
		if err := os.RemoveAll(gen); err != nil {
			return err
		}
	} else if tx.Previous != "" {
		previous, err := generationPath(root, tx.Previous)
		if err != nil {
			return err
		}
		if err := os.RemoveAll(previous); err != nil {
			return err
		}
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	syncDir(root)
	return nil
}
