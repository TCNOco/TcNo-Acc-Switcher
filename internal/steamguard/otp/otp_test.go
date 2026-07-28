package otp

import (
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGenerateVectors(t *testing.T) {
	secret := base64.StdEncoding.EncodeToString([]byte("01234567890123456789"))
	tests := []struct {
		unix int64
		want string
	}{
		{0, "GYTCJ"},
		{30, "TN6YK"},
		{1_700_000_000, "N3FRN"},
	}
	for _, tt := range tests {
		t.Run(time.Unix(tt.unix, 0).UTC().Format(time.RFC3339), func(t *testing.T) {
			got, err := Generate(secret, time.Unix(tt.unix, 0))
			if err != nil {
				t.Fatal(err)
			}
			if got.Value != tt.want {
				t.Fatalf("code = %q, want %q", got.Value, tt.want)
			}
			if got.IntervalStart.Unix() != (tt.unix/30)*30 || got.ExpiresAt.Sub(got.IntervalStart) != 30*time.Second {
				t.Fatalf("interval = %s to %s", got.IntervalStart, got.ExpiresAt)
			}
		})
	}
}

func TestGenerateRolloverAndValidation(t *testing.T) {
	secret := base64.StdEncoding.EncodeToString([]byte("01234567890123456789"))
	before, err := Generate(secret, time.Unix(29, 999_999_999))
	if err != nil {
		t.Fatal(err)
	}
	after, err := Generate(secret, time.Unix(30, 0))
	if err != nil {
		t.Fatal(err)
	}
	if before.IntervalStart.Unix() != 0 || after.IntervalStart.Unix() != 30 || before.Value == after.Value {
		t.Fatalf("rollover codes = %q, %q", before.Value, after.Value)
	}
	for _, secret := range []string{"", "%%%", base64.StdEncoding.EncodeToString([]byte("short"))} {
		if _, err := Generate(secret, time.Unix(0, 0)); err != ErrInvalidSharedSecret {
			t.Fatalf("Generate(%q) error = %v", secret, err)
		}
	}
	if _, err := Generate(secret, time.Unix(-1, 0)); err != ErrInvalidTime {
		t.Fatalf("negative time error = %v", err)
	}
}

func TestGenerateConcurrent(t *testing.T) {
	secret := base64.StdEncoding.EncodeToString([]byte("01234567890123456789"))
	const workers = 64
	var wg sync.WaitGroup
	errCh := make(chan string, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, err := Generate(secret, time.Unix(1_700_000_000, 0))
			if err != nil || len(code.Value) != codeLength || strings.ContainsAny(code.Value, "01ILOZ") {
				errCh <- fmt.Sprintf("code = %q, error = %v", code.Value, err)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for failure := range errCh {
		t.Error(failure)
	}
}
