package steamguard

import (
	"errors"

	"TcNo-Acc-Switcher/internal/steamguard/loginrecord"
	"TcNo-Acc-Switcher/internal/steamguard/mafile"
	"TcNo-Acc-Switcher/internal/steamguard/vault"
	"TcNo-Acc-Switcher/internal/steamguard/vaultrecord"
)

var (
	// ErrNotAuthenticator is returned for an operation that needs authenticator
	// secrets against a record that has none - a login-only account. It is a
	// statement about the record, not a failure: code generation, confirmations,
	// QR approval and maFile export are all legitimately unavailable.
	ErrNotAuthenticator = errors.New("this Steam account has no authenticator stored")

	// ErrRecordPending is a half-finished enrollment. The account exists in the
	// vault but has no usable authenticator yet.
	ErrRecordPending = errors.New("this Steam account's enrollment is unfinished")
)

// loadedRecord is a decrypted vault record of whichever shape it turned out to
// be. Exactly one of Account or Login is meaningful, per Kind.
type loadedRecord struct {
	Kind    vaultrecord.Kind
	Account mafile.Account
	Login   loginrecord.Record
}

// AccountName is the Steam login name, whichever shape holds it.
func (r loadedRecord) AccountName() string {
	if r.Kind == vaultrecord.KindLoginOnly {
		return r.Login.AccountName
	}
	return r.Account.AccountName
}

// AccessToken is the account's Steam bearer token, whichever shape holds it.
// Empty when the record has none.
func (r loadedRecord) AccessToken() string {
	switch r.Kind {
	case vaultrecord.KindLoginOnly:
		return r.Login.AccessToken
	case vaultrecord.KindMaFile:
		if r.Account.Session != nil {
			return r.Account.Session.AccessToken
		}
	}
	return ""
}

// RefreshToken is the long-lived token a lapsed session is renewed from,
// whichever shape holds it. Empty when the record has none, which is the one
// state no renewal can recover from.
func (r loadedRecord) RefreshToken() string {
	switch r.Kind {
	case vaultrecord.KindLoginOnly:
		return r.Login.RefreshToken
	case vaultrecord.KindMaFile:
		if r.Account.Session != nil {
			return r.Account.Session.RefreshToken
		}
	}
	return ""
}

// SessionID is the account's stored web session id, or empty. SDA-imported
// maFiles routinely have none; callers mint one for the operation.
func (r loadedRecord) SessionID() string {
	switch r.Kind {
	case vaultrecord.KindLoginOnly:
		return r.Login.SessionID
	case vaultrecord.KindMaFile:
		if r.Account.Session != nil {
			return r.Account.Session.SessionID
		}
	}
	return ""
}

func (r *loadedRecord) destroy() {
	if r == nil {
		return
	}
	r.Login.Destroy()
	*r = loadedRecord{}
}

// readRecord decrypts a record and reports which shape it holds.
//
// Callers that need an authenticator should keep using accountFromReader, which
// turns the other shapes into ErrNotAuthenticator/ErrRecordPending. This is for
// the few callers that genuinely handle more than one shape.
func readRecord(reader accountRecordReader, recordID string) (loadedRecord, error) {
	raw, err := reader.GetRecord(recordID)
	if err != nil {
		return loadedRecord{}, err
	}
	defer wipe(raw)

	switch vaultrecord.Sniff(raw) {
	case vaultrecord.KindLoginOnly:
		login, err := loginrecord.Decode(raw)
		if err != nil {
			return loadedRecord{}, errors.Join(ErrInvalidImport, err)
		}
		return loadedRecord{Kind: vaultrecord.KindLoginOnly, Login: login}, nil
	case vaultrecord.KindEnrollmentPending:
		return loadedRecord{Kind: vaultrecord.KindEnrollmentPending}, nil
	case vaultrecord.KindMaFile:
		parsed, err := mafile.ParsePlaintext(raw)
		if err != nil {
			return loadedRecord{}, errors.Join(ErrInvalidImport, err)
		}
		return loadedRecord{Kind: vaultrecord.KindMaFile, Account: parsed.Account}, nil
	default:
		// A record written by a newer build. Report it rather than guessing at
		// a shape we do not own.
		return loadedRecord{}, ErrInvalidImport
	}
}

func recordFromVault(v *vault.Vault, recordID string) (loadedRecord, error) {
	return readRecord(v, recordID)
}

// recordKindForSteamID reports the shape stored for steamID64, or KindUnknown
// when the vault holds no record for it.
func recordKindForSteamID(reader interface {
	accountRecordReader
	List() ([]vault.RecordInfo, error)
}, steamID64 string) (vaultrecord.Kind, string, error) {
	records, err := reader.List()
	if err != nil {
		return vaultrecord.KindUnknown, "", err
	}
	for _, record := range records {
		if record.SteamID64 != steamID64 {
			continue
		}
		raw, err := reader.GetRecord(record.ID)
		if err != nil {
			return vaultrecord.KindUnknown, "", err
		}
		kind := vaultrecord.Sniff(raw)
		wipe(raw)
		return kind, record.ID, nil
	}
	return vaultrecord.KindUnknown, "", nil
}
