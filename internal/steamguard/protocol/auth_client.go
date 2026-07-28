package protocol

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	beginCredentialsEndpoint   = "https://api.steampowered.com/IAuthenticationService/BeginAuthSessionViaCredentials/v1"
	updateGuardCodeEndpoint    = "https://api.steampowered.com/IAuthenticationService/UpdateAuthSessionWithSteamGuardCode/v1"
	pollAuthSessionEndpoint    = "https://api.steampowered.com/IAuthenticationService/PollAuthSessionStatus/v1"
	generateAccessEndpoint     = "https://api.steampowered.com/IAuthenticationService/GenerateAccessTokenForApp/v1"
	getAuthSessionInfoEndpoint = "https://api.steampowered.com/IAuthenticationService/GetAuthSessionInfo/v1"
	updateMobileEndpoint       = "https://api.steampowered.com/IAuthenticationService/UpdateAuthSessionWithMobileConfirmation/v1"
	maxAuthResponseBytes       = 64 << 10
	steamResultOK              = 1
)

// AuthenticationClient implements bounded authentication session operations.
type AuthenticationClient struct {
	client  *Client
	entropy io.Reader
}

func NewAuthenticationClient(client *Client) *AuthenticationClient {
	return &AuthenticationClient{client: client, entropy: rand.Reader}
}

func newAuthenticationClientForTest(client *Client, entropy io.Reader) *AuthenticationClient {
	return &AuthenticationClient{client: client, entropy: entropy}
}

func (a *AuthenticationClient) BeginAuthSessionViaCredentials(ctx context.Context, request BeginCredentialsRequest, timeout time.Duration) (BeginCredentialsResult, error) {
	if a == nil || a.client == nil || a.entropy == nil {
		return BeginCredentialsResult{}, protocolError(CodeInvalidRequest, StateInvalid)
	}
	if validationErr := validateBeginCredentialsRequest(request); validationErr != nil {
		return BeginCredentialsResult{}, validationErr
	}
	localID, idErr := a.newLocalSessionID()
	if idErr != nil {
		return BeginCredentialsResult{}, idErr
	}
	message := marshalBeginCredentialsRequest(request)
	response, err := a.call(ctx, beginCredentialsEndpoint, message, timeout)
	wipeBytes(message)
	if err != nil {
		return BeginCredentialsResult{}, err
	}
	defer wipeBytes(response)

	wireResult, parseErr := unmarshalBeginCredentialsResponse(response)
	if parseErr != nil {
		return BeginCredentialsResult{}, parseErr
	}
	session := AuthSession{
		id:           localID,
		clientID:     wireResult.clientID,
		requestID:    wireResult.requestID,
		steamID:      wireResult.steamID,
		pollInterval: wireResult.interval,
		challenges:   append([]AllowedChallenge(nil), wireResult.challenges...),
	}
	state := AuthResultWaiting
	if hasActionableChallenge(session.challenges) {
		state = AuthResultChallengeRequired
	}
	if wireResult.agreementURL != "" {
		state = AuthResultAgreementRequired
	}
	return BeginCredentialsResult{
		State:         state,
		Session:       session,
		AgreementURL:  wireResult.agreementURL,
		ServerMessage: wireResult.serverMessage,
	}, nil
}

func (a *AuthenticationClient) UpdateAuthSessionWithSteamGuardCode(ctx context.Context, request SteamGuardCodeRequest, timeout time.Duration) (ChallengeResult, error) {
	if a == nil || a.client == nil || !validateAuthSession(request.Session) ||
		!validCodeChallenge(request.Type) || !validSteamGuardCode(request.Code) {
		return ChallengeResult{}, protocolError(CodeInvalidRequest, StateInvalid)
	}
	message := marshalSteamGuardCodeRequest(request.Session, request.Code, request.Type)
	response, err := a.callResponse(ctx, updateGuardCodeEndpoint, message, timeout)
	wipeBytes(message)
	if err != nil {
		return ChallengeResult{}, err
	}
	defer wipeBytes(response.Body)
	// A wrong or expired code comes back as HTTP 200 with a non-OK x-eresult and
	// an empty body, which would otherwise read as an accepted challenge. The
	// header is not required, so a response that omits it still goes to the parser.
	if response.HasEResult && response.EResult != steamResultOK {
		return ChallengeResult{}, &Error{Code: CodeSteamResult, State: StateDenied, EResult: response.EResult, HasEResult: true}
	}
	agreementURL, parseErr := unmarshalSteamGuardCodeResponse(response.Body)
	if parseErr != nil {
		return ChallengeResult{}, parseErr
	}
	state := AuthResultChallengeAccepted
	if agreementURL != "" {
		state = AuthResultAgreementRequired
	}
	return ChallengeResult{State: state, AgreementURL: agreementURL}, nil
}

func (a *AuthenticationClient) PollAuthSessionStatus(ctx context.Context, session AuthSession, timeout time.Duration) (PollResult, error) {
	if a == nil || a.client == nil || !validateAuthSession(session) {
		return PollResult{}, protocolError(CodeInvalidRequest, StateInvalid)
	}
	message := marshalPollRequest(session)
	response, err := a.call(ctx, pollAuthSessionEndpoint, message, timeout)
	wipeBytes(message)
	if err != nil {
		return PollResult{}, err
	}
	defer wipeBytes(response)
	wireResult, parseErr := unmarshalPollResponse(response)
	if parseErr != nil {
		return PollResult{}, parseErr
	}
	updatedSession := session
	if wireResult.newClientID != 0 {
		updatedSession.clientID = wireResult.newClientID
	}
	state := AuthResultWaiting
	if wireResult.challengeURL != "" {
		state = AuthResultChallengeRequired
	}
	if wireResult.accessToken != "" || wireResult.refreshToken != "" {
		state = AuthResultAuthorized
	}
	if wireResult.agreementURL != "" {
		state = AuthResultAgreementRequired
	}
	return PollResult{
		State:                state,
		Session:              updatedSession,
		RefreshToken:         wireResult.refreshToken,
		AccessToken:          wireResult.accessToken,
		HadRemoteInteraction: wireResult.hadRemoteInteraction,
		AccountName:          wireResult.accountName,
		GuardData:            wireResult.guardData,
		ChallengeURL:         wireResult.challengeURL,
		AgreementURL:         wireResult.agreementURL,
	}, nil
}

func (a *AuthenticationClient) GenerateAccessTokenForApp(ctx context.Context, request GenerateAccessTokenRequest, timeout time.Duration) (TokenResult, error) {
	if a == nil || a.client == nil || !validAccountSteamID(request.SteamID) ||
		!validOpaqueString(request.RefreshToken, maxTokenBytes) ||
		(request.Renewal != RenewalNone && request.Renewal != RenewalAllow) {
		return TokenResult{}, protocolError(CodeInvalidRequest, StateInvalid)
	}
	message := marshalGenerateAccessTokenRequest(request)
	response, err := a.callResponse(ctx, generateAccessEndpoint, message, timeout)
	wipeBytes(message)
	if err != nil {
		return TokenResult{}, err
	}
	defer wipeBytes(response.Body)
	// Steam refuses a stale or revoked refresh token with HTTP 200, a non-OK
	// x-eresult and an empty body. That is a rejection, not a malformed
	// response, and callers route the two outcomes differently. The header is
	// not required, so a response that omits it still goes through the parser.
	if response.HasEResult && response.EResult != steamResultOK {
		return TokenResult{}, &Error{Code: CodeSteamResult, State: StateDenied, EResult: response.EResult, HasEResult: true}
	}
	result, parseErr := unmarshalGenerateAccessTokenResponse(response.Body)
	if parseErr != nil {
		return TokenResult{}, parseErr
	}
	return result, nil
}

// GetAuthSessionInfo returns bounded requestor details for an explicit QR-login
// approval decision. AccessToken must be a MobileApp access token.
func (a *AuthenticationClient) GetAuthSessionInfo(ctx context.Context, request AuthSessionInfoRequest, timeout time.Duration) (AuthSessionInfo, error) {
	if a == nil || a.client == nil || request.ClientID == 0 || !validOpaqueString(request.AccessToken, maxTokenBytes) {
		return AuthSessionInfo{}, protocolError(CodeInvalidRequest, StateInvalid)
	}
	message := marshalGetAuthSessionInfoRequest(request.ClientID)
	response, err := a.callResponse(ctx, authenticatedEndpoint(getAuthSessionInfoEndpoint, request.AccessToken), message, timeout)
	wipeBytes(message)
	if err != nil {
		return AuthSessionInfo{}, err
	}
	defer wipeBytes(response.Body)
	if resultErr := requireSuccessfulEResult(response); resultErr != nil {
		return AuthSessionInfo{}, resultErr
	}
	result, parseErr := unmarshalGetAuthSessionInfoResponse(response.Body)
	if parseErr != nil {
		return AuthSessionInfo{}, parseErr
	}
	return result, nil
}

func (a *AuthenticationClient) UpdateAuthSessionWithMobileConfirmation(ctx context.Context, request MobileConfirmationRequest, timeout time.Duration) (ChallengeResult, error) {
	if a == nil || a.client == nil || !validOpaqueString(request.AccessToken, maxTokenBytes) || !validAuthVersion(request.Version) || request.ClientID == 0 || !validAccountSteamID(request.SteamID) ||
		len(request.Signature) != 32 || !validPersistence(request.Persistence) {
		return ChallengeResult{}, protocolError(CodeInvalidRequest, StateInvalid)
	}
	message := marshalMobileConfirmationRequest(request)
	response, err := a.callResponse(ctx, authenticatedEndpoint(updateMobileEndpoint, request.AccessToken), message, timeout)
	wipeBytes(message)
	if err != nil {
		return ChallengeResult{}, err
	}
	defer wipeBytes(response.Body)
	if resultErr := requireSuccessfulEResult(response); resultErr != nil {
		return ChallengeResult{}, resultErr
	}
	if len(response.Body) != 0 {
		return ChallengeResult{}, invalidResponse()
	}
	state := AuthResultChallengeDenied
	if request.Confirm {
		state = AuthResultChallengeAccepted
	}
	return ChallengeResult{State: state}, nil
}

func (a *AuthenticationClient) call(ctx context.Context, endpoint string, protobufMessage []byte, timeout time.Duration) ([]byte, error) {
	response, err := a.callResponse(ctx, endpoint, protobufMessage, timeout)
	if err != nil {
		return nil, err
	}
	return response.Body, nil
}

// getResponse issues a read-only IAuthenticationService method. Steam registers
// those for GET only and answers a POST with an HTTP error status, so the
// protobuf payload travels as a query parameter instead of a form body.
func (a *AuthenticationClient) getResponse(ctx context.Context, endpoint string, protobufMessage []byte, timeout time.Duration) (Response, error) {
	query := encodeProtobufForm(protobufMessage)
	defer wipeBytes(query)
	separator := "?"
	if strings.Contains(endpoint, "?") {
		separator = "&"
	}
	return a.client.Do(ctx, Request{
		Method:   http.MethodGet,
		Endpoint: endpoint + separator + string(query),
		Route:    RouteRequest,
		Header: http.Header{
			"Accept": {"application/x-protobuf"},
		},
		Timeout:          timeout,
		MaxResponseBytes: maxAuthResponseBytes,
	})
}

func (a *AuthenticationClient) callResponse(ctx context.Context, endpoint string, protobufMessage []byte, timeout time.Duration) (Response, error) {
	formBody := encodeProtobufForm(protobufMessage)
	defer wipeBytes(formBody)
	response, err := a.client.Do(ctx, Request{
		Method:   http.MethodPost,
		Endpoint: endpoint,
		Route:    RouteRequest,
		Header: http.Header{
			"Accept":       {"application/x-protobuf"},
			"Content-Type": {"application/x-www-form-urlencoded"},
		},
		Body:             formBody,
		Timeout:          timeout,
		MaxResponseBytes: maxAuthResponseBytes,
	})
	if err != nil {
		return Response{}, err
	}
	return response, nil
}

func authenticatedEndpoint(endpoint, accessToken string) string {
	return endpoint + "?access_token=" + url.QueryEscape(accessToken)
}

func requireSuccessfulEResult(response Response) *Error {
	if !response.HasEResult {
		return invalidResponse()
	}
	if response.EResult != steamResultOK {
		return &Error{Code: CodeSteamResult, State: StateDenied, EResult: response.EResult, HasEResult: true}
	}
	return nil
}

func (a *AuthenticationClient) newLocalSessionID() (string, *Error) {
	value := make([]byte, localSessionIDBytes)
	if _, err := io.ReadFull(a.entropy, value); err != nil {
		wipeBytes(value)
		return "", protocolError(CodeEntropy, StateFailed)
	}
	id := base64.RawURLEncoding.EncodeToString(value)
	wipeBytes(value)
	return id, nil
}

func encodeProtobufForm(message []byte) []byte {
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(message)))
	base64.StdEncoding.Encode(encoded, message)
	body := make([]byte, 0, len("input_protobuf_encoded=")+len(encoded)+16)
	body = append(body, "input_protobuf_encoded="...)
	for _, value := range encoded {
		switch value {
		case '+':
			body = append(body, "%2B"...)
		case '/':
			body = append(body, "%2F"...)
		case '=':
			body = append(body, "%3D"...)
		default:
			body = append(body, value)
		}
	}
	wipeBytes(encoded)
	return body
}

func hasActionableChallenge(challenges []AllowedChallenge) bool {
	for _, challenge := range challenges {
		if challenge.Type != ChallengeNone {
			return true
		}
	}
	return false
}
