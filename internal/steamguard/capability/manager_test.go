package capability

import (
	"bytes"
	"errors"
	"sync"
	"testing"
)

// The frontend recognises this refusal by its text, because a message is all
// the error is once it has crossed the Wails binding. Recognising it is what
// lets the Steam Guard window acquire a fresh capability and retry, instead of
// showing the user a failure for a background vault write they never made - see
// isStaleCapabilityError in frontend/src/lib/steamGuardModal.ts. Rewording this
// breaks that silently: nothing the compiler checks connects the two.
func TestInvalidCapabilityMessageIsTheContractWithTheFrontend(t *testing.T) {
	const wanted = "invalid Steam Guard window capability"
	if got := ErrInvalidCapability.Error(); got != wanted {
		t.Fatalf("ErrInvalidCapability = %q, want %q", got, wanted)
	}
}

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

func TestRebindMovesTheGenerationWithoutReissuingTheToken(t *testing.T) {
	m := NewManager()
	binding := Binding{WindowName: "main", AccountID: "7656119", Scope: "modal", LeaseID: "lease", VaultGeneration: "before"}
	token, err := m.Issue(binding)
	if err != nil {
		t.Fatal(err)
	}

	m.Rebind(" after ")

	// The point of the whole exercise: the window holds the same token string
	// across a background write and its next call is authorized against the
	// generation that write committed.
	after := binding
	after.VaultGeneration = "after"
	if err := m.Validate(after, token); err != nil {
		t.Fatalf("validate against the carried generation = %v", err)
	}
	if err := m.Validate(binding, token); !errors.Is(err, ErrInvalidCapability) {
		t.Fatalf("validate against the replaced generation = %v, want ErrInvalidCapability", err)
	}

	// An empty generation is a caller bug, not an instruction to unbind every
	// grant from the vault state it was issued against.
	m.Rebind("")
	if err := m.Validate(after, token); err != nil {
		t.Fatalf("validate after an empty rebind = %v", err)
	}

	// Rebinding must not resurrect what was handed back.
	m.Revoke(token)
	m.Rebind("later")
	if err := m.Validate(Binding{WindowName: "main", AccountID: "7656119", Scope: "modal", LeaseID: "lease", VaultGeneration: "later"}, token); !errors.Is(err, ErrInvalidCapability) {
		t.Fatalf("validate after revoke and rebind = %v, want ErrInvalidCapability", err)
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
