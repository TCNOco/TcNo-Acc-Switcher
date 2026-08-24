package authflow

import (
	"context"
	"runtime"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/protocol"
)

// ProtocolClient adapts protocol.AuthenticationClient to Manager's narrow,
// byte-slice-based credential boundary.
type ProtocolClient struct {
	client *protocol.AuthenticationClient
}

func NewProtocolClient(client *protocol.AuthenticationClient) *ProtocolClient {
	return &ProtocolClient{client: client}
}

func (c *ProtocolClient) Begin(ctx context.Context, request protocol.PasswordCredentialsRequest, password []byte, timeout time.Duration) (protocol.BeginCredentialsResult, error) {
	if c == nil || c.client == nil {
		return protocol.BeginCredentialsResult{}, flowError(ErrorInvalid)
	}
	result, err := c.client.BeginAuthSessionWithPassword(ctx, request, password, timeout)
	runtime.KeepAlive(password)
	return result, err
}

func (c *ProtocolClient) BeginQR(ctx context.Context, request protocol.BeginQRRequest, timeout time.Duration) (protocol.BeginQRResult, error) {
	if c == nil || c.client == nil {
		return protocol.BeginQRResult{}, flowError(ErrorInvalid)
	}
	return c.client.BeginAuthSessionViaQR(ctx, request, timeout)
}

func (c *ProtocolClient) SubmitCode(ctx context.Context, session protocol.AuthSession, challenge protocol.ChallengeType, code []byte, timeout time.Duration) (protocol.ChallengeResult, error) {
	if c == nil || c.client == nil {
		return protocol.ChallengeResult{}, flowError(ErrorInvalid)
	}
	// protocol currently accepts a string. Keep this conversion inside the
	// adapter so the manager never stores or serializes the challenge answer.
	result, err := c.client.UpdateAuthSessionWithSteamGuardCode(ctx, protocol.SteamGuardCodeRequest{
		Session: session,
		Code:    string(code),
		Type:    challenge,
	}, timeout)
	runtime.KeepAlive(code)
	return result, err
}

func (c *ProtocolClient) Poll(ctx context.Context, session protocol.AuthSession, timeout time.Duration) (protocol.PollResult, error) {
	if c == nil || c.client == nil {
		return protocol.PollResult{}, flowError(ErrorInvalid)
	}
	return c.client.PollAuthSessionStatus(ctx, session, timeout)
}
