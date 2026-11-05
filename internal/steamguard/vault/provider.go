package vault

import "strings"

type Provider string

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
