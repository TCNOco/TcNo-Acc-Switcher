// Package mafile parses and writes the Steam Desktop Authenticator maFile format.
// It deliberately does not perform filesystem access.
package mafile

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// MaxInputBytes bounds both plaintext maFiles and legacy encrypted inputs.
	MaxInputBytes            = 1 << 20
	maxJSONDepth             = 32
	maxStringBytes           = 64 << 10
	maxCollectionEntries     = 128
	authenticatorSecretBytes = 20
	maxAccountNameBytes      = 128
	maxMetadataBytes         = 2048
	maxSessionIDBytes        = 256
	maxSessionTokenBytes     = 8192
)

var (
	// ErrWrongPasswordOrCorruptSource intentionally hides decryption details.
	ErrWrongPasswordOrCorruptSource = errors.New("wrong password or corrupt source")
	errInvalidMaFile                = errors.New("invalid maFile")
)

var (
	steamIDMin        = uint64(76561197960265728)
	steamIDMax        = steamIDMin + uint64(^uint32(0))
	deviceIDPattern   = regexp.MustCompile(`^android:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	revocationPattern = regexp.MustCompile(`^R[0-9A-Z]{5,31}$`)
)

// Account is the SDA SteamGuardAccount JSON shape. Unknown fields are never
// retained or emitted.
type Account struct {
	SharedSecret   string       `json:"shared_secret"`
	SerialNumber   string       `json:"serial_number,omitempty"`
	RevocationCode string       `json:"revocation_code,omitempty"`
	URI            string       `json:"uri,omitempty"`
	ServerTime     int64        `json:"server_time,omitempty"`
	AccountName    string       `json:"account_name,omitempty"`
	TokenGID       string       `json:"token_gid,omitempty"`
	IdentitySecret string       `json:"identity_secret"`
	Secret1        string       `json:"secret_1,omitempty"`
	Status         int64        `json:"status,omitempty"`
	DeviceID       string       `json:"device_id"`
	FullyEnrolled  bool         `json:"fully_enrolled,omitempty"`
	Session        *SessionData `json:"Session,omitempty"`
}

// SessionData uses the PascalCase property names emitted by SDA.
type SessionData struct {
	SessionID        string `json:"SessionID,omitempty"`
	SteamLogin       string `json:"SteamLogin,omitempty"`
	SteamLoginSecure string `json:"SteamLoginSecure,omitempty"`
	WebCookie        string `json:"WebCookie,omitempty"`
	OAuthToken       string `json:"OAuthToken,omitempty"`
	SteamID          uint64 `json:"SteamID,omitempty"`
	AccessToken      string `json:"AccessToken,omitempty"`
	RefreshToken     string `json:"RefreshToken,omitempty"`
}

// ParseResult contains an account and the names of fields discarded during
// parsing. Names never include field values.
type ParseResult struct {
	Account         Account
	DiscardedFields []string
}

// ExportOptions controls whether bearer tokens are present in the output.
// Tokens are omitted unless IncludeTokens is true.
type ExportOptions struct {
	IncludeTokens bool
}

// ParsePlaintext parses one unencrypted SDA maFile.
func ParsePlaintext(data []byte) (ParseResult, error) {
	if len(data) == 0 || len(data) > MaxInputBytes || !utf8.Valid(data) {
		return ParseResult{}, errInvalidMaFile
	}
	value, err := decodeLimitedJSON(data)
	if err != nil {
		return ParseResult{}, errInvalidMaFile
	}
	root, ok := value.(map[string]any)
	if !ok {
		return ParseResult{}, errInvalidMaFile
	}
	account, discarded, err := accountFromObject(root)
	if err != nil {
		return ParseResult{}, err
	}
	return ParseResult{Account: account, DiscardedFields: discarded}, nil
}

// ImportLegacyEncrypted imports SDA's historical AES-CBC encrypted maFile
// representation. The supplied manifest is the adjacent manifest.json bytes;
// sourceFilename must be the source basename, not a path.
func ImportLegacyEncrypted(ciphertext, manifest []byte, sourceFilename, password string) (ParseResult, error) {
	if len(ciphertext) == 0 || len(ciphertext) > MaxInputBytes || len(manifest) == 0 || len(manifest) > MaxInputBytes || !utf8.Valid(ciphertext) || !utf8.Valid(manifest) || !safeFilename(sourceFilename) {
		return ParseResult{}, ErrWrongPasswordOrCorruptSource
	}
	entry, ok := legacyManifestEntry(manifest, sourceFilename)
	if !ok {
		return ParseResult{}, ErrWrongPasswordOrCorruptSource
	}
	salt, err := strictBase64(entry.Salt)
	if err != nil || len(salt) != 8 {
		return ParseResult{}, ErrWrongPasswordOrCorruptSource
	}
	iv, err := strictBase64(entry.IV)
	if err != nil || len(iv) != aes.BlockSize {
		return ParseResult{}, ErrWrongPasswordOrCorruptSource
	}
	encoded := strings.TrimSpace(string(ciphertext))
	sealed, err := strictBase64(encoded)
	if err != nil || len(sealed) == 0 || len(sealed)%aes.BlockSize != 0 {
		return ParseResult{}, ErrWrongPasswordOrCorruptSource
	}
	key := pbkdf2.Key([]byte(password), salt, legacyPBKDF2Iterations, legacyKeyBytes, sha1.New)
	defer wipeBytes(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return ParseResult{}, ErrWrongPasswordOrCorruptSource
	}
	decrypted := make([]byte, len(sealed))
	defer wipeBytes(decrypted)
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(decrypted, sealed)
	plain, ok := unpadPKCS7(decrypted)
	if !ok {
		return ParseResult{}, ErrWrongPasswordOrCorruptSource
	}
	result, err := ParsePlaintext(plain)
	if err != nil || !validSteamID(entry.SteamID) || entry.Filename != strconv.FormatUint(entry.SteamID, 10)+".maFile" || !steamIDsAgree(entry.SteamID, result.Account.Session) || !filenameAgrees(sourceFilename, result.Account.Session) {
		return ParseResult{}, ErrWrongPasswordOrCorruptSource
	}
	return result, nil
}

// ExportPlaintext returns compact, canonical SDA-compatible JSON. It never
// writes a file and it omits all web-session credentials by default.
func ExportPlaintext(account Account, options ExportOptions) ([]byte, error) {
	if err := validateAccount(account); err != nil {
		return nil, err
	}
	output := account
	if output.Session != nil && !options.IncludeTokens {
		if output.Session.SteamID == 0 {
			output.Session = nil
		} else {
			output.Session = &SessionData{SteamID: output.Session.SteamID}
		}
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		return nil, errInvalidMaFile
	}
	return encoded, nil
}

func accountFromObject(root map[string]any) (Account, []string, error) {
	// SDA maFiles have no schema envelope. Reject versioned or typed records so
	// pending/vault records cannot be mistaken for an active authenticator.
	if _, versioned := root["version"]; versioned {
		return Account{}, nil, errInvalidMaFile
	}
	if _, typed := root["kind"]; typed {
		return Account{}, nil, errInvalidMaFile
	}
	known := map[string]bool{
		"shared_secret": true, "serial_number": true, "revocation_code": true,
		"uri": true, "server_time": true, "account_name": true, "token_gid": true,
		"identity_secret": true, "secret_1": true, "status": true, "device_id": true,
		"fully_enrolled": true, "Session": true,
	}
	discarded := unknownNames(root, known, "")
	var a Account
	var ok bool
	if a.SharedSecret, ok = requiredString(root, "shared_secret"); !ok {
		return Account{}, nil, errInvalidMaFile
	}
	if a.IdentitySecret, ok = requiredString(root, "identity_secret"); !ok {
		return Account{}, nil, errInvalidMaFile
	}
	if a.DeviceID, ok = requiredString(root, "device_id"); !ok {
		return Account{}, nil, errInvalidMaFile
	}
	if strings.HasPrefix(a.DeviceID, "android%3A") || strings.HasPrefix(a.DeviceID, "android%3a") {
		a.DeviceID = "android:" + a.DeviceID[len("android%3A"):]
	}
	if a.SerialNumber, ok = optionalString(root, "serial_number"); !ok {
		return Account{}, nil, errInvalidMaFile
	}
	if a.RevocationCode, ok = optionalString(root, "revocation_code"); !ok {
		return Account{}, nil, errInvalidMaFile
	}
	if a.URI, ok = optionalString(root, "uri"); !ok {
		return Account{}, nil, errInvalidMaFile
	}
	if a.AccountName, ok = optionalString(root, "account_name"); !ok {
		return Account{}, nil, errInvalidMaFile
	}
	if a.TokenGID, ok = optionalString(root, "token_gid"); !ok {
		return Account{}, nil, errInvalidMaFile
	}
	if a.Secret1, ok = optionalString(root, "secret_1"); !ok {
		return Account{}, nil, errInvalidMaFile
	}
	if a.ServerTime, ok = optionalInt(root, "server_time"); !ok {
		return Account{}, nil, errInvalidMaFile
	}
	if a.Status, ok = optionalInt(root, "status"); !ok {
		return Account{}, nil, errInvalidMaFile
	}
	if a.FullyEnrolled, ok = optionalBool(root, "fully_enrolled"); !ok {
		return Account{}, nil, errInvalidMaFile
	}
	if raw, exists := root["Session"]; exists && raw != nil {
		sessionObject, isObject := raw.(map[string]any)
		if !isObject {
			return Account{}, nil, errInvalidMaFile
		}
		session, sessionDiscarded, err := sessionFromObject(sessionObject)
		if err != nil {
			return Account{}, nil, err
		}
		a.Session = &session
		discarded = append(discarded, sessionDiscarded...)
	}
	if err := validateAccount(a); err != nil {
		return Account{}, nil, err
	}
	return a, discarded, nil
}

func sessionFromObject(object map[string]any) (SessionData, []string, error) {
	known := map[string]bool{"SessionID": true, "SteamLogin": true, "SteamLoginSecure": true, "WebCookie": true, "OAuthToken": true, "SteamID": true, "AccessToken": true, "RefreshToken": true}
	discarded := unknownNames(object, known, "Session.")
	var s SessionData
	var ok bool
	if s.SessionID, ok = optionalString(object, "SessionID"); !ok {
		return SessionData{}, nil, errInvalidMaFile
	}
	if s.SteamLogin, ok = optionalString(object, "SteamLogin"); !ok {
		return SessionData{}, nil, errInvalidMaFile
	}
	if s.SteamLoginSecure, ok = optionalString(object, "SteamLoginSecure"); !ok {
		return SessionData{}, nil, errInvalidMaFile
	}
	if s.WebCookie, ok = optionalString(object, "WebCookie"); !ok {
		return SessionData{}, nil, errInvalidMaFile
	}
	if s.OAuthToken, ok = optionalString(object, "OAuthToken"); !ok {
		return SessionData{}, nil, errInvalidMaFile
	}
	if s.AccessToken, ok = optionalString(object, "AccessToken"); !ok {
		return SessionData{}, nil, errInvalidMaFile
	}
	if s.RefreshToken, ok = optionalString(object, "RefreshToken"); !ok {
		return SessionData{}, nil, errInvalidMaFile
	}
	if raw, exists := object["SteamID"]; exists {
		number, isNumber := raw.(json.Number)
		if !isNumber {
			return SessionData{}, nil, errInvalidMaFile
		}
		id, err := strconv.ParseUint(string(number), 10, 64)
		if err != nil {
			return SessionData{}, nil, errInvalidMaFile
		}
		s.SteamID = id
	}
	if s.SteamID != 0 && !validSteamID(s.SteamID) {
		return SessionData{}, nil, errInvalidMaFile
	}
	return s, discarded, nil
}

func validateAccount(account Account) error {
	if !account.FullyEnrolled {
		return errInvalidMaFile
	}
	if err := validSecret(account.SharedSecret); err != nil {
		return errInvalidMaFile
	}
	if err := validSecret(account.IdentitySecret); err != nil {
		return errInvalidMaFile
	}
	if account.Secret1 != "" && validSecret(account.Secret1) != nil {
		return errInvalidMaFile
	}
	if !deviceIDPattern.MatchString(account.DeviceID) {
		return errInvalidMaFile
	}
	if !validDisplayText(account.AccountName, maxAccountNameBytes) ||
		!validDisplayText(account.SerialNumber, maxMetadataBytes) ||
		!validDisplayText(account.URI, maxMetadataBytes) ||
		!validDisplayText(account.TokenGID, maxMetadataBytes) {
		return errInvalidMaFile
	}
	if account.RevocationCode != "" && !revocationPattern.MatchString(account.RevocationCode) {
		return errInvalidMaFile
	}
	if account.Session != nil && !validSession(account.Session) {
		return errInvalidMaFile
	}
	return nil
}

func validSecret(value string) error {
	decoded, err := strictBase64(value)
	if err != nil {
		return errInvalidMaFile
	}
	defer wipeBytes(decoded)
	if len(decoded) != authenticatorSecretBytes {
		return errInvalidMaFile
	}
	return nil
}

func validSession(session *SessionData) bool {
	if session == nil {
		return true
	}
	if session.SteamID != 0 && !validSteamID(session.SteamID) {
		return false
	}
	fields := []struct {
		value string
		limit int
	}{
		{session.SessionID, maxSessionIDBytes},
		{session.SteamLogin, maxSessionTokenBytes},
		{session.SteamLoginSecure, maxSessionTokenBytes},
		{session.WebCookie, maxSessionTokenBytes},
		{session.OAuthToken, maxSessionTokenBytes},
		{session.AccessToken, maxSessionTokenBytes},
		{session.RefreshToken, maxSessionTokenBytes},
	}
	hasCredential := false
	for _, field := range fields {
		if field.value == "" {
			continue
		}
		hasCredential = true
		if !validVisibleASCII(field.value, field.limit) {
			return false
		}
	}
	return !hasCredential || session.SteamID != 0
}

func validVisibleASCII(value string, limit int) bool {
	if len(value) > limit {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < 0x21 || value[i] > 0x7e {
			return false
		}
	}
	return true
}

func validDisplayText(value string, limit int) bool {
	if len(value) > limit {
		return false
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func strictBase64(value string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || base64.StdEncoding.EncodeToString(decoded) != value {
		return nil, errors.New("bad base64")
	}
	return decoded, nil
}

func optionalString(object map[string]any, key string) (string, bool) {
	raw, exists := object[key]
	if !exists || raw == nil {
		return "", true
	}
	value, ok := raw.(string)
	return value, ok
}
func requiredString(object map[string]any, key string) (string, bool) {
	value, ok := optionalString(object, key)
	return value, ok && value != ""
}
func optionalInt(object map[string]any, key string) (int64, bool) {
	raw, exists := object[key]
	if !exists {
		return 0, true
	}
	number, ok := raw.(json.Number)
	if !ok {
		return 0, false
	}
	value, err := strconv.ParseInt(string(number), 10, 64)
	return value, err == nil
}
func optionalBool(object map[string]any, key string) (bool, bool) {
	raw, exists := object[key]
	if !exists {
		return false, true
	}
	value, ok := raw.(bool)
	return value, ok
}
func unknownNames(object map[string]any, known map[string]bool, prefix string) []string {
	var names []string
	for key := range object {
		if !known[key] {
			names = append(names, prefix+key)
		}
	}
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return names
}

func decodeLimitedJSON(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	value, err := readJSONValue(decoder, 0)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errInvalidMaFile
	}
	return value, nil
}
func readJSONValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > maxJSONDepth {
		return nil, errInvalidMaFile
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, errInvalidMaFile
	}
	switch typed := token.(type) {
	case json.Delim:
		switch typed {
		case '{':
			object := make(map[string]any)
			for decoder.More() {
				if len(object) >= maxCollectionEntries {
					return nil, errInvalidMaFile
				}
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, errInvalidMaFile
				}
				key, ok := keyToken.(string)
				if !ok || len(key) > maxStringBytes || strings.ContainsRune(key, utf8.RuneError) {
					return nil, errInvalidMaFile
				}
				if _, exists := object[key]; exists {
					return nil, errInvalidMaFile
				}
				value, err := readJSONValue(decoder, depth+1)
				if err != nil {
					return nil, err
				}
				object[key] = value
			}
			if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
				return nil, errInvalidMaFile
			}
			return object, nil
		case '[':
			array := make([]any, 0)
			for decoder.More() {
				if len(array) >= maxCollectionEntries {
					return nil, errInvalidMaFile
				}
				value, err := readJSONValue(decoder, depth+1)
				if err != nil {
					return nil, err
				}
				array = append(array, value)
			}
			if end, err := decoder.Token(); err != nil || end != json.Delim(']') {
				return nil, errInvalidMaFile
			}
			return array, nil
		default:
			return nil, errInvalidMaFile
		}
	case string:
		if len(typed) > maxStringBytes || strings.ContainsRune(typed, utf8.RuneError) {
			return nil, errInvalidMaFile
		}
		return typed, nil
	case json.Number, bool, nil:
		return typed, nil
	default:
		return nil, errInvalidMaFile
	}
}

type manifestEntry struct {
	Filename, Salt, IV string
	SteamID            uint64
}

func legacyManifestEntry(data []byte, filename string) (manifestEntry, bool) {
	value, err := decodeLimitedJSON(data)
	if err != nil {
		return manifestEntry{}, false
	}
	root, ok := value.(map[string]any)
	if !ok {
		return manifestEntry{}, false
	}
	encrypted, encryptedOK := root["encrypted"].(bool)
	if !encryptedOK || !encrypted {
		return manifestEntry{}, false
	}
	entries, ok := root["entries"].([]any)
	if !ok {
		return manifestEntry{}, false
	}
	var matched manifestEntry
	matchCount := 0
	for _, raw := range entries {
		object, ok := raw.(map[string]any)
		if !ok {
			return manifestEntry{}, false
		}
		name, nameOK := requiredString(object, "filename")
		salt, saltOK := requiredString(object, "encryption_salt")
		iv, ivOK := requiredString(object, "encryption_iv")
		idRaw, idExists := object["steamid"]
		idNumber, idOK := idRaw.(json.Number)
		id, err := strconv.ParseUint(string(idNumber), 10, 64)
		if !nameOK || !saltOK || !ivOK || !idExists || !idOK || err != nil || !safeFilename(name) {
			return manifestEntry{}, false
		}
		if name == filename {
			matched = manifestEntry{Filename: name, Salt: salt, IV: iv, SteamID: id}
			matchCount++
		}
	}
	return matched, matchCount == 1
}
func safeFilename(name string) bool {
	if name == "" || name != strings.TrimSpace(name) || strings.ContainsAny(name, `/\\:`) || strings.Contains(name, "..") || strings.ToLower(name) == "manifest.json" {
		return false
	}
	return strings.HasSuffix(name, ".maFile") && strings.TrimSuffix(name, ".maFile") != ""
}
func filenameAgrees(name string, session *SessionData) bool {
	if session == nil || session.SteamID == 0 {
		return true
	}
	return name == strconv.FormatUint(session.SteamID, 10)+".maFile"
}
func steamIDsAgree(manifestID uint64, session *SessionData) bool {
	return session == nil || session.SteamID == 0 || manifestID == session.SteamID
}

func validSteamID(id uint64) bool { return id >= steamIDMin && id <= steamIDMax }

func wipeBytes(data []byte) {
	for i := range data {
		data[i] = 0
	}
	runtime.KeepAlive(data)
}

func unpadPKCS7(data []byte) ([]byte, bool) {
	if len(data) == 0 || len(data)%aes.BlockSize != 0 {
		return nil, false
	}
	n := int(data[len(data)-1])
	if n == 0 || n > aes.BlockSize || n > len(data) {
		return nil, false
	}
	for _, value := range data[len(data)-n:] {
		if int(value) != n {
			return nil, false
		}
	}
	return data[:len(data)-n], true
}
