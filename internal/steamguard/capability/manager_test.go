package capability

import (
	"bytes"
	"errors"
	"sync"
	"testing"
)

func TestIssueValidateAndRevoke(t *testing.T) {
	m := NewManager()
	m.random = bytes.NewReader(bytes.Repeat([]byte{0x7a}, tokenBytes))
	binding := Binding{WindowName: "main", AccountID: "7656119", Scope: "modal", LeaseID: "lease", VaultGeneration: "generation"}
	token, err := m.Issue(binding)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Validate(binding, token); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		binding Binding
		token   string
	}{
		{Binding{WindowName: "other", AccountID: binding.AccountID, Scope: binding.Scope, LeaseID: binding.LeaseID, VaultGeneration: binding.VaultGeneration}, token},
		{binding, token + "x"},
		{Binding{}, token},
		{binding, ""},
	} {
		if err := m.Validate(test.binding, test.token); !errors.Is(err, ErrInvalidCapability) {
			t.Fatalf("Validate(%#v) error = %v", test.binding, err)
		}
	}
	m.Revoke(token)
	if err := m.Validate(binding, token); !errors.Is(err, ErrInvalidCapability) {
		t.Fatalf("validate after revoke = %v", err)
	}
}

func TestIssueRotatesAndConcurrentValidation(t *testing.T) {
	m := NewManager()
	firstBinding := Binding{WindowName: "main", AccountID: "one", Scope: "modal", LeaseID: "lease-one"}
	first, err := m.Issue(firstBinding)
	if err != nil {
		t.Fatal(err)
	}
	secondBinding := Binding{WindowName: "main", AccountID: "two", Scope: "modal", LeaseID: "lease-two"}
	second, err := m.Issue(secondBinding)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("capability did not rotate")
	}
	if err := m.Validate(firstBinding, first); err != nil {
		t.Fatalf("parallel capability was unexpectedly rotated = %v", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := m.Validate(secondBinding, second); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	m.RevokeWindow("main")
	if err := m.Validate(secondBinding, second); !errors.Is(err, ErrInvalidCapability) {
		t.Fatalf("window revoke = %v", err)
	}
	m.RevokeAll()
}
