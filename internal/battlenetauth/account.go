package battlenetauth

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var ErrInvalid = errors.New("invalid Battle.net authenticator data")
var serialPattern = regexp.MustCompile(`^(US|EU|KR|TW|CN)-[0-9]{4}-[0-9]{4}-[0-9]{4}$`)
var compactSerialPattern = regexp.MustCompile(`^(US|EU|KR|TW|CN)[0-9]{12}$`)

func NormalizeSerial(raw string) (string, error) {
	serial := strings.ToUpper(strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, raw))
	if serialPattern.MatchString(serial) {
		return serial, nil
	}
	if !compactSerialPattern.MatchString(serial) {
		return "", ErrInvalid
	}
	return serial[:2] + "-" + serial[2:6] + "-" + serial[6:10] + "-" + serial[10:], nil
}

type Account struct {
	Version              int    `json:"version"`
	Serial               string `json:"serial"`
	Label                string `json:"label,omitempty"`
	AccountName          string `json:"accountName,omitempty"`
	RestoreCode          string `json:"restoreCode,omitempty"`
	DeviceSecret         string `json:"deviceSecret"`
	AccountLink          string `json:"accountLink,omitempty"`
	Pending              bool   `json:"pending,omitempty"`
	BrowserProfile       string `json:"browserProfile,omitempty"`
	RequiresAccountSetup bool   `json:"requireHealup,omitempty"`
}

func (Account) String() string   { return "Battle.net authenticator (secret redacted)" }
func (Account) GoString() string { return "Battle.net authenticator (secret redacted)" }

func (a Account) Validate() error {
	if a.BrowserProfile != "" {
		if len(a.BrowserProfile) > 64 || !strings.HasPrefix(a.BrowserProfile, "battlenet-") {
			return ErrInvalid
		}
		for _, c := range a.BrowserProfile {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return ErrInvalid
			}
		}
	}
	if a.Version != 1 || !serialPattern.MatchString(a.Serial) || len(a.Label) > 200 || len(a.AccountName) > 320 || len(a.AccountLink) > 320 {
		return ErrInvalid
	}
	secret, err := hex.DecodeString(a.DeviceSecret)
	defer clear(secret)
	if err != nil || len(secret) != 20 {
		return ErrInvalid
	}
	if a.RestoreCode != "" {
		if len(a.RestoreCode) != 10 {
			return ErrInvalid
		}
		for _, c := range a.RestoreCode {
			if !strings.ContainsRune("0123456789ABCDEFGHJKLMNPQRSTUVWXYZ", c) {
				return ErrInvalid
			}
		}
	}
	for _, text := range []string{a.Label, a.AccountName, a.AccountLink} {
		for _, c := range text {
			if c < 32 || c == 127 {
				return ErrInvalid
			}
		}
	}
	return nil
}

func (a Account) DisplayName() string {
	if a.Label != "" && a.Label != a.Serial {
		return a.Label
	}
	if a.AccountName != "" {
		return a.AccountName
	}
	return a.Serial
}

func (a Account) Code(now time.Time) (string, int64, error) {
	if err := a.Validate(); err != nil {
		return "", 0, err
	}
	if now.Unix() < 0 {
		return "", 0, ErrInvalid
	}
	secret, _ := hex.DecodeString(a.DeviceSecret)
	defer clear(secret)
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(now.Unix()/30))
	mac := hmac.New(sha1.New, secret)
	mac.Write(counter[:])
	digest := mac.Sum(nil)
	defer clear(digest)
	offset := digest[len(digest)-1] & 15
	value := binary.BigEndian.Uint32(digest[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%08d", value%100000000), (now.Unix()/30 + 1) * 30, nil
}

func (a Account) URI() (string, error) {
	if err := a.Validate(); err != nil {
		return "", err
	}
	secret, _ := hex.DecodeString(a.DeviceSecret)
	defer clear(secret)
	q := url.Values{"secret": {base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)}, "issuer": {"Battle.net"}, "digits": {"8"}, "algorithm": {"SHA1"}, "period": {"30"}}
	return "otpauth://totp/Battle.net:" + a.Serial + "?" + q.Encode(), nil
}

func ParseURI(raw string) (Account, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "otpauth" || u.Host != "totp" || u.User != nil || u.Fragment != "" {
		return Account{}, ErrInvalid
	}
	q := u.Query()
	for _, values := range q {
		if len(values) != 1 {
			return Account{}, ErrInvalid
		}
	}
	if q.Get("digits") != "8" || (q.Get("period") != "" && q.Get("period") != "30") || (q.Get("algorithm") != "" && !strings.EqualFold(q.Get("algorithm"), "SHA1")) {
		return Account{}, ErrInvalid
	}
	if issuer := q.Get("issuer"); issuer != "" && !strings.EqualFold(issuer, "Battle.net") {
		return Account{}, ErrInvalid
	}
	label := strings.TrimPrefix(u.Path, "/")
	label = strings.TrimPrefix(label, "Battle.net:")
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimRight(q.Get("secret"), "=")))
	defer clear(secret)
	if err != nil {
		return Account{}, ErrInvalid
	}
	serial, err := NormalizeSerial(label)
	if err != nil {
		return Account{}, err
	}
	a := Account{Version: 1, Serial: serial, DeviceSecret: hex.EncodeToString(secret)}
	return a, a.Validate()
}

func ParseAccount(raw []byte) (Account, error) {
	if len(raw) > 1<<20 {
		return Account{}, ErrInvalid
	}
	var a Account
	if err := json.Unmarshal(raw, &a); err != nil {
		return Account{}, ErrInvalid
	}
	return a, a.Validate()
}
