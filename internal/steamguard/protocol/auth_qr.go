package protocol

import (
	"context"
	"math"
	"time"
)

const beginQREndpoint = "https://api.steampowered.com/IAuthenticationService/BeginAuthSessionViaQR/v1"

// BeginQRRequest starts a session that is authorised by a Steam mobile app
// scanning the challenge URL, rather than by a password. Nothing identifies the
// account here: which account signs in is decided by whoever scans, and is only
// known once the poll answers.
type BeginQRRequest struct {
	DeviceFriendlyName string
	Platform           PlatformType
	Device             DeviceDetails
	WebsiteID          string
}

// BeginQRResult reports the session and the URL its QR image must encode.
type BeginQRResult struct {
	State        AuthResultState
	Session      AuthSession
	ChallengeURL string
	Version      int32
}

type beginQRWireResult struct {
	clientID     uint64
	challengeURL string
	requestID    []byte
	interval     time.Duration
	challenges   []AllowedChallenge
	version      int32
	seen         uint64
}

// BeginAuthSessionViaQR opens an unauthenticated session for a QR sign-in.
//
// The session it returns carries no SteamID. A credentials session is bound to
// one account from the first call, and everything downstream leans on that; a QR
// session cannot be, because the account is whichever one scans the code. The
// session is marked so only polling accepts it - see validatePollableSession -
// and the caller has to check the account the poll reports before it trusts the
// tokens for anyone.
func (a *AuthenticationClient) BeginAuthSessionViaQR(ctx context.Context, request BeginQRRequest, timeout time.Duration) (BeginQRResult, error) {
	if a == nil || a.client == nil || a.entropy == nil {
		return BeginQRResult{}, protocolError(CodeInvalidRequest, StateInvalid)
	}
	if validationErr := validateBeginQRRequest(request); validationErr != nil {
		return BeginQRResult{}, validationErr
	}
	localID, idErr := a.newLocalSessionID()
	if idErr != nil {
		return BeginQRResult{}, idErr
	}
	message := marshalBeginQRRequest(request)
	response, err := a.callResponse(ctx, beginQREndpoint, message, timeout)
	wipeBytes(message)
	if err != nil {
		return BeginQRResult{}, err
	}
	defer wipeBytes(response.Body)
	if response.HasEResult && response.EResult != steamResultOK {
		return BeginQRResult{}, &Error{Code: CodeSteamResult, State: StateDenied, EResult: response.EResult, HasEResult: true}
	}

	wireResult, parseErr := unmarshalBeginQRResponse(response.Body)
	if parseErr != nil {
		return BeginQRResult{}, parseErr
	}
	return BeginQRResult{
		State: AuthResultWaiting,
		Session: AuthSession{
			id:           localID,
			clientID:     wireResult.clientID,
			requestID:    wireResult.requestID,
			pollInterval: wireResult.interval,
			challenges:   append([]AllowedChallenge(nil), wireResult.challenges...),
			viaQR:        true,
		},
		ChallengeURL: wireResult.challengeURL,
		Version:      wireResult.version,
	}, nil
}

func marshalBeginQRRequest(request BeginQRRequest) []byte {
	device := marshalDeviceDetails(request.Device)
	message := make([]byte, 0, 512)
	message = appendStringField(message, 1, request.DeviceFriendlyName)
	message = appendVarintField(message, 2, uint64(request.Platform))
	message = appendBytesField(message, 3, device)
	message = appendStringField(message, 4, request.WebsiteID)
	wipeBytes(device)
	return message
}

func unmarshalBeginQRResponse(data []byte) (beginQRWireResult, *Error) {
	var result beginQRWireResult
	var challengeURL []byte
	decoder := protobufDecoder{data: data}
	for {
		field, ok := decoder.next()
		if !ok {
			break
		}
		switch field.number {
		case 1:
			value, valid := fieldVarint(field)
			if !valid || !markSingleton(&result.seen, field.number) {
				return beginQRWireResult{}, invalidResponseDetail("qr_begin_client_id_field")
			}
			result.clientID = value
		case 2:
			value, valid := fieldBytes(field, maxChallengeURLBytes)
			if !valid || !markSingleton(&result.seen, field.number) {
				return beginQRWireResult{}, invalidResponseDetail("qr_begin_challenge_url_field")
			}
			challengeURL = value
		case 3:
			value, valid := fieldBytes(field, maxRequestIDBytes)
			if !valid || len(value) == 0 || !markSingleton(&result.seen, field.number) {
				return beginQRWireResult{}, invalidResponseDetail("qr_begin_request_id_field")
			}
			result.requestID = value
		case 4:
			if field.typeID != wireFixed32 || !markSingleton(&result.seen, field.number) {
				return beginQRWireResult{}, invalidResponseDetail("qr_begin_interval_field")
			}
			seconds := math.Float32frombits(field.fixed32)
			if math.IsNaN(float64(seconds)) || math.IsInf(float64(seconds), 0) || seconds < 0.25 || seconds > 60 {
				return beginQRWireResult{}, invalidResponseDetail("qr_begin_interval_range")
			}
			result.interval = time.Duration(float64(seconds) * float64(time.Second))
		case 5:
			value, valid := fieldBytes(field, 512)
			if !valid || len(result.challenges) >= 8 {
				return beginQRWireResult{}, invalidResponseDetail("qr_begin_confirmation_field")
			}
			challenge, parseErr := unmarshalAllowedChallenge(value)
			if parseErr != nil {
				return beginQRWireResult{}, parseErr
			}
			result.challenges = append(result.challenges, challenge)
		case 6:
			value, valid := fieldVarint(field)
			if !valid || value > math.MaxInt32 || !markSingleton(&result.seen, field.number) {
				return beginQRWireResult{}, invalidResponseDetail("qr_begin_version_field")
			}
			result.version = int32(value)
		default:
			return beginQRWireResult{}, invalidResponseDetail("qr_begin_unknown_field")
		}
	}
	if !decoder.validEnd() {
		return beginQRWireResult{}, invalidResponseDetail("qr_begin_trailing_bytes")
	}
	if result.clientID == 0 {
		return beginQRWireResult{}, invalidResponseDetail("qr_begin_missing_client_id")
	}
	if len(result.requestID) == 0 {
		return beginQRWireResult{}, invalidResponseDetail("qr_begin_missing_request_id")
	}
	if result.interval == 0 {
		return beginQRWireResult{}, invalidResponseDetail("qr_begin_missing_interval")
	}
	// The URL is the whole point of the call, and it is about to be turned into
	// an image the user is asked to trust. Steam's own format is the only one
	// accepted, and it has to name the session that was just opened.
	if !validChallengeURL(string(challengeURL)) || !challengeURLNamesClient(string(challengeURL), result.clientID) {
		return beginQRWireResult{}, invalidResponseDetail("qr_begin_challenge_url")
	}
	result.requestID = append([]byte(nil), result.requestID...)
	result.challengeURL = string(challengeURL)
	return result, nil
}
