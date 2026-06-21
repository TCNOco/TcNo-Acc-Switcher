package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"TcNo-Acc-Switcher/internal/appclient"
)

const crowdinAPIURL = "https://tcno.co/Projects/AccSwitcher/api/crowdin/"

// CrowdinProofReader is a project member with proofreader languages.
type CrowdinProofReader struct {
	Name      string `json:"name"`
	Languages string `json:"languages"`
}

// CrowdinTranslatorsList is returned to the SPA for the translators modal.
type CrowdinTranslatorsList struct {
	ProofReaders []CrowdinProofReader `json:"proofReaders"`
	Translators  []string             `json:"translators"`
}

type crowdinAPIResponse struct {
	ProofReaders map[string]string `json:"ProofReaders"`
	Translators  []string          `json:"Translators"`
}

// GetCrowdinTranslators fetches Crowdin project members from tcno.co.
func (*PlatformService) GetCrowdinTranslators() (CrowdinTranslatorsList, error) {
	if appclient.IsOfflineMode() {
		return CrowdinTranslatorsList{}, errors.New("offline mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, crowdinAPIURL, nil)
	if err != nil {
		return CrowdinTranslatorsList{}, err
	}

	resp, err := appclient.Shared.Do(req)
	if err != nil {
		return CrowdinTranslatorsList{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return CrowdinTranslatorsList{}, fmt.Errorf("crowdin api: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return CrowdinTranslatorsList{}, err
	}

	var raw crowdinAPIResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return CrowdinTranslatorsList{}, err
	}

	proof := make([]CrowdinProofReader, 0, len(raw.ProofReaders))
	for name, langs := range raw.ProofReaders {
		proof = append(proof, CrowdinProofReader{Name: name, Languages: langs})
	}
	sort.Slice(proof, func(i, j int) bool {
		return proof[i].Name < proof[j].Name
	})

	translators := append([]string(nil), raw.Translators...)
	sort.Strings(translators)

	return CrowdinTranslatorsList{
		ProofReaders: proof,
		Translators:  translators,
	}, nil
}
