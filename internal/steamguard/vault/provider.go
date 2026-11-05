package vault

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Provider string

type providerRecoveryFile struct {
	Version      int    `json:"version"`
	OuterVersion int    `json:"outerVersion"`
	Committed    bool   `json:"committed"`
	Payload      []byte `json:"payload"`
}

const (
	ProviderSteam     Provider = "steam"
	ProviderBattleNet Provider = "battlenet"
)

func ProviderIdentity(provider Provider, subject string) (string, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" || len(subject) > 128 || strings.ContainsAny(subject, ":\x00\r\n") {
		return "", ErrInvalidFormat
	}
	switch provider {
	case ProviderSteam:
		return subject, nil
	case ProviderBattleNet:
		return "battlenet:" + subject, nil
	default:
		return "", ErrInvalidFormat
	}
}

func recordProvider(identity string) Provider {
	if strings.HasPrefix(identity, "battlenet:") {
		return ProviderBattleNet
	}
	return ProviderSteam
}

func filterProvider(records []RecordInfo, provider Provider) []RecordInfo {
	out := make([]RecordInfo, 0, len(records))
	for _, record := range records {
		if recordProvider(record.SteamID64) == provider {
			out = append(out, record)
		}
	}
	return out
}

func (v *Vault) ListProviderRecords(provider Provider) ([]RecordInfo, error) {
	if provider != ProviderSteam && provider != ProviderBattleNet {
		return nil, ErrInvalidFormat
	}
	records, err := v.ListAllRecords()
	return filterProvider(records, provider), err
}

func (v *Vault) PutProviderRecord(provider Provider, subject string, plaintext []byte) (string, error) {
	identity, err := ProviderIdentity(provider, subject)
	if err != nil {
		return "", err
	}
	return v.PutRecord(identity, plaintext)
}

type BattleNetRecoveryPayload struct {
	ID        string
	Plaintext []byte
}

func (BattleNetRecoveryPayload) String() string   { return "Battle.net recovery payload (redacted)" }
func (BattleNetRecoveryPayload) GoString() string { return "Battle.net recovery payload (redacted)" }

func (v *Vault) CaptureProviderRecord(provider Provider, capture func(func([]byte) error) (string, []byte, error)) (string, error) {
	if provider != ProviderBattleNet || capture == nil {
		return "", ErrInvalidFormat
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	receiptName := ""
	err := v.withKeysLocked(func(key, outerKey []byte) error {
		ring, err := v.loadKeyring(key, outerKey)
		if err != nil {
			return err
		}
		saveRecovery := func(plain []byte) error {
			if receiptName != "" || len(plain) > 128<<10 {
				return ErrInvalidFormat
			}
			recovery, err := seal(key, plain, aad(FormatVersion, v.header.VaultID, "", "", "battlenet-recovery"))
			if err != nil {
				return err
			}
			raw, err := json.Marshal(recovery)
			if err != nil {
				return err
			}
			defer wipe(raw)
			id, err := randomID()
			if err != nil {
				return err
			}
			name := "battlenet-recovery-" + id + ".json"
			payload, err := sealOuterFile(outerKey, raw, v.header.OuterVersion, outerAAD(v.header.VaultID, "", name))
			if err != nil {
				return err
			}
			defer wipe(payload)
			receipt := providerRecoveryFile{Version: 1, OuterVersion: v.header.OuterVersion, Payload: payload}
			encoded, err := json.Marshal(receipt)
			if err != nil {
				return err
			}
			defer wipe(encoded)
			if err := atomicWrite(filepath.Join(v.root, name), encoded, 0o600, v.opts.hardener); err != nil {
				return err
			}
			receiptName = name
			return nil
		}
		subject, plain, err := capture(saveRecovery)
		defer wipe(plain)
		if err != nil {
			return err
		}
		identity, err := ProviderIdentity(provider, subject)
		if err != nil {
			return err
		}
		for _, record := range ring.Records {
			if record.SteamID64 == identity {
				return ErrAlreadyExists
			}
		}
		if receiptName == "" {
			if err := saveRecovery(plain); err != nil {
				return err
			}
		}
		staged := map[string][]byte{}
		if _, err := v.stageRecordLocked(key, &ring, staged, identity, plain); err != nil {
			return err
		}
		files, err := v.collectRecordFiles(ring, staged, outerKey)
		if err != nil {
			return err
		}
		if err := v.commitGenerationLocked(key, outerKey, v.header, ring, files); err != nil {
			return err
		}
		return v.completeBattleNetRecoveryLocked(receiptName)
	})
	return receiptName, err
}

func (v *Vault) completeBattleNetRecoveryLocked(id string) error {
	if len(id) > 128 || filepath.Base(id) != id || strings.ContainsAny(id, "/\\:") || !strings.HasPrefix(id, "battlenet-recovery-") || !strings.HasSuffix(id, ".json") {
		return ErrInvalidFormat
	}
	var receipt providerRecoveryFile
	if err := readJSONFile(filepath.Join(v.root, id), 512<<10, &receipt); err != nil {
		return err
	}
	if receipt.Version != 1 {
		return ErrInvalidFormat
	}
	receipt.Committed = true
	raw, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	defer wipe(raw)
	return atomicWrite(filepath.Join(v.root, id), raw, 0o600, v.opts.hardener)
}

func (v *Vault) CompleteBattleNetRecovery(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.withKeysLocked(func([]byte, []byte) error { return v.completeBattleNetRecoveryLocked(id) })
}

func (v *Vault) BattleNetRecoveryPayloads() ([]BattleNetRecoveryPayload, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := []BattleNetRecoveryPayload{}
	err := v.withKeysLocked(func(key, outerKey []byte) error {
		entries, err := os.ReadDir(v.root)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasPrefix(name, "battlenet-recovery-") || !strings.HasSuffix(name, ".json") {
				continue
			}
			if len(out) >= 100 || !entry.Type().IsRegular() {
				return ErrInvalidFormat
			}
			var receipt providerRecoveryFile
			if err := readJSONFile(filepath.Join(v.root, name), 512<<10, &receipt); err != nil {
				return err
			}
			if receipt.Version != 1 {
				return ErrInvalidFormat
			}
			if receipt.Committed {
				continue
			}
			inner, err := openOuterFile(outerKey, receipt.Payload, receipt.OuterVersion, outerAAD(v.header.VaultID, "", name))
			if err != nil {
				return err
			}
			var env envelope
			err = json.Unmarshal(inner, &env)
			wipe(inner)
			if err != nil {
				return ErrInvalidFormat
			}
			plain, err := openEnvelope(key, env, aad(FormatVersion, v.header.VaultID, "", "", "battlenet-recovery"))
			if err != nil {
				return err
			}
			out = append(out, BattleNetRecoveryPayload{ID: name, Plaintext: plain})
		}
		return nil
	})
	if err != nil {
		for _, raw := range out {
			wipe(raw.Plaintext)
		}
		return nil, err
	}
	return out, nil
}
