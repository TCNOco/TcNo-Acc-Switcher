package battlenetauth

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"sort"
	"strings"

	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/scrypt"
)

const MaxImportSize = 8 << 20

func decodeObject(raw []byte) (map[string]json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > MaxImportSize {
		return nil, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, ErrInvalid
	}
	object := map[string]json.RawMessage{}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return nil, ErrInvalid
		}
		name, ok := key.(string)
		if !ok {
			return nil, ErrInvalid
		}
		if _, duplicate := object[name]; duplicate {
			return nil, ErrInvalid
		}
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return nil, ErrInvalid
		}
		object[name] = value
	}
	if _, err := d.Token(); err != nil {
		return nil, ErrInvalid
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, ErrInvalid
	}
	return object, nil
}

func textField(o map[string]json.RawMessage, name string) string {
	var v string
	_ = json.Unmarshal(o[name], &v)
	return v
}
func intField(o map[string]json.RawMessage, name string) int {
	var v int
	_ = json.Unmarshal(o[name], &v)
	return v
}

func Import(raw []byte, password string) ([]Account, error) {
	if len(raw) > MaxImportSize {
		return nil, ErrInvalid
	}
	if strings.HasPrefix(strings.TrimSpace(string(raw)), "otpauth:") {
		a, err := ParseURI(string(raw))
		if err != nil {
			return nil, err
		}
		return []Account{a}, nil
	}
	o, err := decodeObject(raw)
	if err != nil {
		return nil, err
	}
	if _, encrypted := o["ciphertext"]; encrypted {
		plain, err := decryptPackage(o, password)
		if err != nil {
			return nil, err
		}
		defer clear(plain)
		o, err = decodeObject(plain)
		if err != nil {
			return nil, err
		}
	}
	if entries, ok := o["entries"]; ok {
		if intField(o, "version") != 1 {
			return nil, ErrInvalid
		}
		items, err := decodeObject(entries)
		if err != nil || len(items) > 1000 {
			return nil, ErrInvalid
		}
		keys := make([]string, 0, len(items))
		for key := range items {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make([]Account, 0, len(items))
		for _, key := range keys {
			a, err := parseExternal(items[key])
			serial, serialErr := NormalizeSerial(key)
			if err != nil || serialErr != nil || a.Serial != serial {
				return nil, ErrInvalid
			}
			out = append(out, a)
		}
		return out, nil
	}
	a, err := parseExternalObject(o)
	if err != nil {
		return nil, err
	}
	return []Account{a}, nil
}

func parseExternal(raw []byte) (Account, error) {
	o, err := decodeObject(raw)
	if err != nil {
		return Account{}, err
	}
	return parseExternalObject(o)
}

func parseExternalObject(o map[string]json.RawMessage) (Account, error) {
	if raw, ok := o["version"]; ok {
		var version int
		if json.Unmarshal(raw, &version) != nil || version != 1 {
			return Account{}, ErrInvalid
		}
	}
	serial, err := NormalizeSerial(textField(o, "serial"))
	if err != nil {
		return Account{}, err
	}
	a := Account{Version: 1, Serial: serial, RestoreCode: strings.ToUpper(textField(o, "restoreCode")), DeviceSecret: strings.ToLower(textField(o, "deviceSecret")), Label: textField(o, "label")}
	a.AccountName = textField(o, "accountName")
	a.RequiresAccountSetup = accountSetupRequired(o)
	for _, encoded := range []string{textField(o, "base32Secret"), textField(o, "totpUrl")} {
		if encoded == "" {
			continue
		}
		uri := encoded
		if !strings.HasPrefix(uri, "otpauth:") {
			uri = "otpauth://totp/Battle.net:" + a.Serial + "?secret=" + encoded + "&digits=8&period=30&algorithm=SHA1"
		}
		other, err := ParseURI(uri)
		if err != nil || other.Serial != a.Serial || (a.DeviceSecret != "" && other.DeviceSecret != a.DeviceSecret) {
			return Account{}, ErrInvalid
		}
		a.DeviceSecret = other.DeviceSecret
	}
	return a, a.Validate()
}

func decryptPackage(o map[string]json.RawMessage, password string) ([]byte, error) {
	if password == "" || len(password) > 4096 {
		return nil, ErrInvalid
	}
	if format := intField(o, "format"); format != 0 && format != 2 {
		return nil, ErrInvalid
	}
	decode := func(name string) ([]byte, error) { return base64.StdEncoding.Strict().DecodeString(textField(o, name)) }
	salt, err := decode("salt")
	if err != nil || len(salt) != 16 {
		return nil, ErrInvalid
	}
	nonce, err := decode("nonce")
	if err != nil || len(nonce) != 12 {
		return nil, ErrInvalid
	}
	data, err := decode("ciphertext")
	if err != nil || len(data) < 16 {
		return nil, ErrInvalid
	}
	var key []byte
	switch textField(o, "kdf") {
	case "scrypt":
		params, err := decodeObject(o["scrypt"])
		if err != nil {
			return nil, err
		}
		n, r, p := intField(params, "n"), intField(params, "r"), intField(params, "p")
		if n < 16384 || n > 262144 || n&(n-1) != 0 || r < 1 || r > 8 || p < 1 || p > 4 || int64(n)*int64(r)*128 > 256<<20 {
			return nil, ErrInvalid
		}
		key, err = scrypt.Key([]byte(password), salt, n, r, p, 32)
		if err != nil {
			return nil, ErrInvalid
		}
	case "", "pbkdf2":
		iterations := intField(o, "kdf_iterations")
		if iterations == 0 {
			if intField(o, "format") == 0 {
				iterations = 100000
			} else {
				iterations = 600000
			}
		}
		if iterations < 100000 || iterations > 2000000 {
			return nil, ErrInvalid
		}
		key = pbkdf2.Key([]byte(password), salt, iterations, 32, sha256.New)
	default:
		return nil, ErrInvalid
	}
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrInvalid
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrInvalid
	}
	plain, err := aead.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, ErrInvalid
	}
	return plain, nil
}

func Export(accounts []Account, password string) ([]byte, error) {
	if len(accounts) == 0 || len(accounts) > 1000 || len(password) < 1 || len(password) > 4096 {
		return nil, ErrInvalid
	}
	entries := map[string]any{}
	for _, a := range accounts {
		if err := a.Validate(); err != nil {
			return nil, err
		}
		if _, exists := entries[a.Serial]; exists {
			return nil, ErrInvalid
		}
		entry := map[string]any{"serial": a.Serial, "deviceSecret": a.DeviceSecret, "restoreCode": a.RestoreCode, "label": a.Label}
		if a.AccountName != "" {
			entry["accountName"] = a.AccountName
		}
		if a.RequiresAccountSetup {
			entry["requireHealup"] = true
		}
		entries[a.Serial] = entry
	}
	plain, err := json.Marshal(map[string]any{"version": 1, "entries": entries})
	if err != nil {
		return nil, err
	}
	defer clear(plain)
	salt, nonce := make([]byte, 16), make([]byte, 12)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	key, err := scrypt.Key([]byte(password), salt, 131072, 8, 1, 32)
	if err != nil {
		return nil, err
	}
	defer clear(key)
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	return json.Marshal(map[string]any{"format": 2, "kdf": "scrypt", "scrypt": map[string]int{"n": 131072, "r": 8, "p": 1}, "salt": base64.StdEncoding.EncodeToString(salt), "nonce": base64.StdEncoding.EncodeToString(nonce), "ciphertext": base64.StdEncoding.EncodeToString(aead.Seal(nil, nonce, plain, nil))})
}
