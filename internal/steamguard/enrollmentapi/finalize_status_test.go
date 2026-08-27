package enrollmentapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Steam's status 1 is EResult OK and means the authenticator now exists on the
// account. It does not always arrive with the separate success flag, and the
// response defines no want_more field at all, so requiring either of those
// rejects a completed enrollment.
func TestFinalizeTreatsStatusOneAsComplete(t *testing.T) {
	cases := map[string][]byte{
		"status only":           appendVarint(appendVarint(nil, 3, testUnix+1), 4, 1),
		"status with success":   appendVarint(appendVarint(appendVarint(nil, 1, 1), 3, testUnix+1), 4, 1),
		"status with success 0": appendVarint(appendVarint(appendVarint(nil, 1, 0), 3, testUnix+1), 4, 1),
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			pending := testPending()
			defer pending.Destroy()
			transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
				return response(http.StatusOK, body, nil), nil
			})
			result, err := testClient(transport, nil).FinalizeAddAuthenticator(context.Background(), FinalizeRequest{
				Pending:           pending,
				RequestID:         append([]byte(nil), pending.RequestID...),
				ConfirmationCode:  []byte("8Y36B"),
				AuthenticatorTime: testUnix,
			}, time.Second)
			if err != nil {
				t.Fatalf("status 1 reported as an error: %v", err)
			}
			if result.State != StateComplete {
				t.Fatalf("state = %q, want %q", result.State, StateComplete)
			}
		})
	}
}

// A genuine refusal must still be refused, and a mapped one stays a state
// rather than an error so the flow can tell the user what to do.
func TestFinalizeStillReportsRefusals(t *testing.T) {
	pending := testPending()
	defer pending.Destroy()
	body := appendVarint(appendVarint(nil, 3, testUnix+1), 4, 89)
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, body, nil), nil
	})
	result, err := testClient(transport, nil).FinalizeAddAuthenticator(context.Background(), FinalizeRequest{
		Pending: pending, RequestID: append([]byte(nil), pending.RequestID...),
		ConfirmationCode: []byte("8Y36B"), AuthenticatorTime: testUnix,
	}, time.Second)
	if err != nil {
		t.Fatalf("a mapped refusal should be a state, not an error: %v", err)
	}
	if result.State != StateConfirmationCodeRejected {
		t.Fatalf("state = %q, want %q", result.State, StateConfirmationCodeRejected)
	}
}

// An unmapped failure status is still an error, and it has to name the code:
// without it every refusal reads identically in a log.
func TestFinalizeUnmappedStatusNamesTheCode(t *testing.T) {
	pending := testPending()
	defer pending.Destroy()
	body := appendVarint(appendVarint(nil, 3, testUnix+1), 4, 4242)
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, body, nil), nil
	})
	_, err := testClient(transport, nil).FinalizeAddAuthenticator(context.Background(), FinalizeRequest{
		Pending: pending, RequestID: append([]byte(nil), pending.RequestID...),
		ConfirmationCode: []byte("8Y36B"), AuthenticatorTime: testUnix,
	}, time.Second)
	if !errors.Is(err, ErrSteamRejected) {
		t.Fatalf("err = %v, want ErrSteamRejected", err)
	}
	if !strings.Contains(err.Error(), "4242") {
		t.Fatalf("error %q does not name the result code", err.Error())
	}
}
