package battlenetauth

import "encoding/json"

const maxResponseSize = 65536

type recoveryResponse struct {
	Version        int    `json:"version"`
	Kind           string `json:"kind"`
	BrowserProfile string `json:"browserProfile,omitempty"`
	Serial         string `json:"serial,omitempty"`
	RestoreCode    string `json:"restoreCode,omitempty"`
	Response       []byte `json:"response"`
}

func EncodeRecovery(response []byte, profile, serial, restoreCode string) ([]byte, error) {
	if len(response) > maxResponseSize+1 {
		return nil, ErrInvalid
	}
	return json.Marshal(recoveryResponse{Version: 1, Kind: "battlenet-response", BrowserProfile: profile, Serial: serial, RestoreCode: restoreCode, Response: response})
}

func RecoverAccount(raw []byte) (Account, error) {
	if a, err := ParseAccount(raw); err == nil {
		return a, nil
	}
	if len(raw) > 128<<10 {
		return Account{}, ErrInvalid
	}
	var saved recoveryResponse
	if err := json.Unmarshal(raw, &saved); err != nil || saved.Version != 1 || saved.Kind != "battlenet-response" || len(saved.Response) > maxResponseSize+1 {
		return Account{}, ErrInvalid
	}
	defer clear(saved.Response)
	o, err := decodeObject(saved.Response)
	if err != nil {
		return Account{}, err
	}
	if setupOnlyResponse(o) {
		return Account{}, ErrAccountVerification
	}
	if saved.Serial != "" {
		o["serial"], _ = json.Marshal(saved.Serial)
		o["restoreCode"], _ = json.Marshal(saved.RestoreCode)
	}
	a, err := parseExternalObject(o)
	if err != nil {
		if setupOnlyResponse(o) {
			return Account{}, ErrAccountVerification
		}
		return Account{}, err
	}
	a.Pending = true
	a.BrowserProfile = saved.BrowserProfile
	return a, a.Validate()
}

func setupOnlyResponse(o map[string]json.RawMessage) bool {
	return accountSetupRequired(o) && len(o["serial"]) == 0 && len(o["restoreCode"]) == 0 && len(o["deviceSecret"]) == 0
}
