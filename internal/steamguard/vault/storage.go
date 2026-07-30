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
	Slots        []keySlot        `json:"slots"`
	OuterVersion int              `json:"outerVersion,omitempty"`
	OuterProof   envelope         `json:"outerProof,omitempty"`
	Recovery     *recoveryWrapper `json:"recovery,omitempty"`
}

// keySlot is one way to open the vault. Slots are alternatives: any single slot
// opens it. Every factor within a slot is required, which makes the slot the
// unit that expresses "password AND security key" — enrolling those as two
// slots would instead mean "either one alone", which looks identical in a list
// and is the easiest way to ship a vault that is weaker than it appears.
type keySlot struct {
	ID       string       `json:"id"`
	Label    string       `json:"label"`
	Factors  []slotFactor `json:"factors"`
	VaultKey envelope     `json:"vaultKey"`
}

// slotFactor is one required input to a slot's key. Salt is per factor. KDF is
// present only for factors derived from something the user types, and absent
// for factors that already carry full entropy.
type slotFactor struct {
	Type      string     `json:"type"`
	Salt      string     `json:"salt"`
	KDF       *KDFParams `json:"kdf,omitempty"`
	KeyfileID string     `json:"keyfileId,omitempty"`

	// Security-key descriptors. None of this is secret, and all of it travels
	// with a backup so an enrolled key still opens a copy on another machine.
	// UV is fixed at enrolment: asserting with a different user-verification
	// setting yields different bytes from the authenticator, so changing it
	// would silently stop the slot opening.
	CredentialID string `json:"credentialId,omitempty"`
	RPID         string `json:"rpId,omitempty"`
	UV           bool   `json:"uv,omitempty"`
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

// validRPID accepts a relying-party identifier as a hostname and nothing else.
// The value is handed to the platform WebAuthn call as a UTF-16 string, and an
// interior NUL panics inside the driver rather than failing as a bad header —
// so a vault restored from a tampered backup would look valid right up until it
// took the app down. It is also one of the fields slotAAD separates with NULs.
func validRPID(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-' || r == '.':
		default:
			return false
		}
	}
	return true
}

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
