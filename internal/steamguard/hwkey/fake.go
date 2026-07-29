package hwkey

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"slices"
)

// Fake is a deterministic Authenticator for tests. It derives the same secret a
// real key would in the one way that matters here: the same credential always
// produces the same bytes, and a different credential never does.
//
// It also models the part that made multiple keys fail in the real driver: a
// physical key holds only the credentials it issued itself, so a Fake asked
// about a credential another Fake created reports that no key was found, rather
// than quietly returning bytes for it.
type Fake struct {
	Seed         []byte
	Absent       bool
	FailEvaluate error

	issued []string
}

func (f *Fake) Available(context.Context) (bool, string) {
	if f.Absent {
		return false, "no security key attached"
	}
	return true, ""
}

func (f *Fake) Enroll(_ context.Context, rpID, userName string) (Credential, error) {
	if f.Absent {
		return Credential{}, ErrNoDevice
	}
	mac := hmac.New(sha256.New, f.Seed)
	mac.Write([]byte(rpID))
	mac.Write([]byte(userName))
	// Enrolling twice on one key has to yield two credentials, the way a real
	// authenticator does, or a test cannot enrol two keys at all.
	mac.Write([]byte{byte(len(f.issued))})
	cred := Credential{
		ID:   base64.RawURLEncoding.EncodeToString(mac.Sum(nil)),
		RPID: rpID,
		UV:   true,
	}
	f.issued = append(f.issued, cred.ID)
	return cred, nil
}

func (f *Fake) Evaluate(_ context.Context, creds []Credential) (Credential, []byte, error) {
	if f.Absent {
		return Credential{}, nil, ErrNoDevice
	}
	if f.FailEvaluate != nil {
		return Credential{}, nil, f.FailEvaluate
	}
	for _, cred := range creds {
		if !slices.Contains(f.issued, cred.ID) {
			continue
		}
		mac := hmac.New(sha256.New, f.Seed)
		mac.Write([]byte(cred.ID))
		mac.Write([]byte(cred.RPID))
		if cred.UV {
			mac.Write([]byte{1})
		}
		mac.Write(SaltFor(cred))
		return cred, mac.Sum(nil)[:SecretLength], nil
	}
	return Credential{}, nil, ErrNoDevice
}
