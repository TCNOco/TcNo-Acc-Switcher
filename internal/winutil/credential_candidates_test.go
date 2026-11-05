package winutil

import (
	"errors"
	"reflect"
	"testing"
)

func TestReadCredentialCandidatesKeepsDistinctAliases(t *testing.T) {
	target := "credentials.example"
	prefixed := legacyGenericTargetPrefix + target
	var reads []string
	credentials, err := readCredentialCandidates(prefixed, func(candidate string) (Credential, error) {
		reads = append(reads, candidate)
		return Credential{TargetName: candidate, Blob: []byte(candidate)}, nil
	})
	if err != nil || len(credentials) != 2 || !reflect.DeepEqual(reads, []string{prefixed, target}) {
		t.Fatalf("credentials=%+v reads=%v err=%v", credentials, reads, err)
	}
	if credentials[0].TargetName != prefixed || credentials[1].TargetName != target {
		t.Fatalf("aliases were reordered: %+v", credentials)
	}
}

func TestReadCredentialCandidatesSkipsMissingAliases(t *testing.T) {
	target := "credentials.example"
	credentials, err := readCredentialCandidates(target, func(candidate string) (Credential, error) {
		if candidate == target {
			return Credential{}, ErrCredentialNotFound
		}
		return Credential{TargetName: candidate, Blob: []byte("present")}, nil
	})
	if err != nil || len(credentials) != 1 || credentials[0].TargetName != legacyGenericTargetPrefix+target {
		t.Fatalf("credentials=%+v err=%v", credentials, err)
	}
}

func TestReadCredentialCandidatesMissingReturnsSentinel(t *testing.T) {
	credentials, err := readCredentialCandidates("credentials.example", func(string) (Credential, error) {
		return Credential{}, ErrCredentialNotFound
	})
	if credentials != nil || !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("credentials=%+v err=%v", credentials, err)
	}
}

func TestReadCredentialCandidatesClearsPartialBlobsOnFailure(t *testing.T) {
	first, partial := []byte("first"), []byte("partial")
	failure := errors.New("credential access denied")
	credentials, err := readCredentialCandidates("credentials.example", func(candidate string) (Credential, error) {
		if candidate == "credentials.example" {
			return Credential{TargetName: candidate, Blob: first}, nil
		}
		return Credential{Blob: partial}, failure
	})
	if credentials != nil || !errors.Is(err, failure) {
		t.Fatalf("credentials=%+v err=%v", credentials, err)
	}
	for _, blob := range [][]byte{first, partial} {
		for _, value := range blob {
			if value != 0 {
				t.Fatal("partial credential blob was retained after failure")
			}
		}
	}
}

func TestReadCredentialCandidatesDeduplicatesNativeTarget(t *testing.T) {
	credentials, err := readCredentialCandidates("credentials.example", func(string) (Credential, error) {
		return Credential{TargetName: "credentials.example", Blob: []byte("copy")}, nil
	})
	if err != nil || len(credentials) != 1 || string(credentials[0].Blob) != "copy" {
		t.Fatalf("credentials=%+v err=%v", credentials, err)
	}
}
