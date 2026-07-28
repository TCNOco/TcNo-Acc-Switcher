package authflow

import (
	"strings"
	"testing"

	"TcNo-Acc-Switcher/internal/steamguard/protocol"
)

// A protocol failure has to name its cause: "protocol failed" alone reads as
// transient, so the user retries an attempt that can never succeed.
func TestProtocolFailureNamesItsCause(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name  string
		cause *protocol.Error
		want  string
	}{
		{
			name:  "http status",
			cause: &protocol.Error{Code: protocol.CodeHTTPStatus, StatusCode: 405},
			want:  "HTTP 405",
		},
		{
			name:  "wrong password",
			cause: &protocol.Error{Code: protocol.CodeSteamResult, EResult: 5, HasEResult: true},
			want:  "rejected the account name or password",
		},
		{
			name:  "unnamed steam result",
			cause: &protocol.Error{Code: protocol.CodeSteamResult, EResult: 3, HasEResult: true},
			want:  "result 3",
		},
		{
			name:  "no detail at all",
			cause: &protocol.Error{Code: protocol.CodeInvalidResponse},
			want:  "invalid_response",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			err := classifyClientError(testCase.cause)
			if err.Kind != ErrorProtocol {
				t.Fatalf("kind = %q, want %q", err.Kind, ErrorProtocol)
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("message = %q, want it to contain %q", err.Error(), testCase.want)
			}
		})
	}
}

// Rate limiting keeps its own kind so callers can honor Retry-After instead of
// treating it as an unexplained protocol failure.
func TestRateLimitKeepsRetryAfter(t *testing.T) {
	t.Parallel()

	err := classifyClientError(&protocol.Error{
		Code:          protocol.CodeRateLimited,
		RetryAfter:    5,
		HasRetryAfter: true,
	})
	if err.Kind != ErrorRateLimited || !err.HasRetryAfter || err.RetryAfter != 5 {
		t.Fatalf("error = %#v", err)
	}
}
