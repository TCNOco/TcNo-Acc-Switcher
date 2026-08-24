package protocol

import (
	"math"
	"time"
)

type beginCredentialsWireResult struct {
	clientID      uint64
	requestID     []byte
	interval      time.Duration
	challenges    []AllowedChallenge
	steamID       uint64
	agreementURL  string
	serverMessage string
}

type pollWireResult struct {
	newClientID          uint64
	challengeURL         string
	refreshToken         string
	accessToken          string
	hadRemoteInteraction bool
	accountName          string
	guardData            string
	agreementURL         string
}

func marshalGetAuthSessionInfoRequest(clientID uint64) []byte {
	return appendVarintField(nil, 1, clientID)
}

// marshalDeviceDetails encodes CAuthentication_DeviceDetails, which both begin
// calls embed. Shared so the two cannot drift: Steam matches the device on a QR
// session against the one that started it.
func marshalDeviceDetails(device DeviceDetails) []byte {
	encoded := make([]byte, 0, 256)
	encoded = appendStringField(encoded, 1, device.FriendlyName)
	encoded = appendVarintField(encoded, 2, uint64(device.Platform))
	encoded = appendVarintField(encoded, 3, uint64(device.OSType))
	if device.GamingDeviceType != 0 {
		encoded = appendVarintField(encoded, 4, uint64(device.GamingDeviceType))
	}
	if device.ClientCount != 0 {
		encoded = appendVarintField(encoded, 5, uint64(device.ClientCount))
	}
	if len(device.MachineID) != 0 {
		encoded = appendBytesField(encoded, 6, device.MachineID)
	}
	return appendVarintField(encoded, 7, uint64(device.App))
}

func marshalBeginCredentialsRequest(request BeginCredentialsRequest) []byte {
	device := marshalDeviceDetails(request.Device)

	message := make([]byte, 0, 1024)
	message = appendStringField(message, 1, request.DeviceFriendlyName)
	message = appendStringField(message, 2, request.AccountName)
	message = appendStringField(message, 3, request.EncryptedPassword)
	message = appendVarintField(message, 4, request.EncryptionTimestamp)
	message = appendVarintField(message, 5, boolVarint(request.RememberLogin))
	message = appendVarintField(message, 6, uint64(request.Platform))
	message = appendVarintField(message, 7, uint64(request.Persistence))
	message = appendStringField(message, 8, request.WebsiteID)
	message = appendBytesField(message, 9, device)
	if request.GuardData != "" {
		message = appendStringField(message, 10, request.GuardData)
	}
	message = appendVarintField(message, 11, uint64(request.Language))
	message = appendVarintField(message, 12, uint64(request.QoSLevel))
	wipeBytes(device)
	return message
}

func marshalSteamGuardCodeRequest(session AuthSession, code string, challenge ChallengeType) []byte {
	message := make([]byte, 0, 64)
	message = appendVarintField(message, 1, session.clientID)
	message = appendFixed64Field(message, 2, session.steamID)
	message = appendStringField(message, 3, code)
	return appendVarintField(message, 4, uint64(challenge))
}

func marshalPollRequest(session AuthSession) []byte {
	message := make([]byte, 0, 64)
	message = appendVarintField(message, 1, session.clientID)
	return appendBytesField(message, 2, session.requestID)
}

func marshalGenerateAccessTokenRequest(request GenerateAccessTokenRequest) []byte {
	message := make([]byte, 0, len(request.RefreshToken)+32)
	message = appendStringField(message, 1, request.RefreshToken)
	message = appendFixed64Field(message, 2, request.SteamID)
	return appendVarintField(message, 3, uint64(request.Renewal))
}

func marshalMobileConfirmationRequest(request MobileConfirmationRequest) []byte {
	message := make([]byte, 0, 80)
	message = appendVarintField(message, 1, uint64(request.Version))
	message = appendVarintField(message, 2, request.ClientID)
	message = appendFixed64Field(message, 3, request.SteamID)
	message = appendBytesField(message, 4, request.Signature)
	message = appendVarintField(message, 5, boolVarint(request.Confirm))
	return appendVarintField(message, 6, uint64(request.Persistence))
}

func unmarshalBeginCredentialsResponse(data []byte) (beginCredentialsWireResult, *Error) {
	var result beginCredentialsWireResult
	var agreementURL []byte
	var serverMessage []byte
	var seen uint64
	decoder := protobufDecoder{data: data}
	for {
		field, ok := decoder.next()
		if !ok {
			break
		}
		switch field.number {
		case 1:
			value, valid := fieldVarint(field)
			if !valid || !markSingleton(&seen, field.number) {
				return beginCredentialsWireResult{}, invalidResponseDetail("begin_client_id_field")
			}
			result.clientID = value
		case 2:
			value, valid := fieldBytes(field, maxRequestIDBytes)
			if !valid || len(value) == 0 || !markSingleton(&seen, field.number) {
				return beginCredentialsWireResult{}, invalidResponseDetail("begin_request_id_field")
			}
			result.requestID = value
		case 3:
			if field.typeID != wireFixed32 || !markSingleton(&seen, field.number) {
				return beginCredentialsWireResult{}, invalidResponseDetail("begin_interval_field")
			}
			seconds := math.Float32frombits(field.fixed32)
			if math.IsNaN(float64(seconds)) || math.IsInf(float64(seconds), 0) || seconds < 0.25 || seconds > 60 {
				return beginCredentialsWireResult{}, invalidResponseDetail("begin_interval_range")
			}
			result.interval = time.Duration(float64(seconds) * float64(time.Second))
		case 4:
			value, valid := fieldBytes(field, 512)
			if !valid || len(result.challenges) >= 8 {
				return beginCredentialsWireResult{}, invalidResponseDetail("begin_confirmation_field")
			}
			challenge, parseErr := unmarshalAllowedChallenge(value)
			if parseErr != nil {
				return beginCredentialsWireResult{}, parseErr
			}
			result.challenges = append(result.challenges, challenge)
		case 5:
			value, valid := fieldVarint(field)
			if !valid || !markSingleton(&seen, field.number) {
				return beginCredentialsWireResult{}, invalidResponseDetail("begin_steamid_field")
			}
			result.steamID = value
		case 6:
			value, valid := fieldBytes(field, maxTokenBytes)
			if !valid || (len(value) != 0 && !validOpaqueBytes(value, maxTokenBytes)) || !markSingleton(&seen, field.number) {
				return beginCredentialsWireResult{}, invalidResponseDetail("begin_weak_token_field")
			}
		case 7:
			value, valid := fieldBytes(field, maxAgreementURLBytes)
			if !valid || !markSingleton(&seen, field.number) {
				return beginCredentialsWireResult{}, invalidResponseDetail("begin_agreement_field")
			}
			agreementURL = value
		case 8:
			value, valid := fieldBytes(field, maxMessageBytes)
			if !valid || !validProtocolText(value, maxMessageBytes, true) || !markSingleton(&seen, field.number) {
				return beginCredentialsWireResult{}, invalidResponseDetail("begin_server_message_field")
			}
			serverMessage = value
		default:
			return beginCredentialsWireResult{}, invalidResponseDetail("begin_unknown_field")
		}
	}
	if !decoder.validEnd() {
		return beginCredentialsWireResult{}, invalidResponseDetail("begin_trailing_bytes")
	}
	// Steam answers a refused sign-in with an empty body, which lands here with
	// no client ID, so this is the label a rejected credential surfaces under
	// when the EResult header is missing too.
	if result.clientID == 0 {
		return beginCredentialsWireResult{}, invalidResponseDetail("begin_missing_client_id")
	}
	if len(result.requestID) == 0 {
		return beginCredentialsWireResult{}, invalidResponseDetail("begin_missing_request_id")
	}
	if result.interval == 0 {
		return beginCredentialsWireResult{}, invalidResponseDetail("begin_missing_interval")
	}
	if !validAccountSteamID(result.steamID) {
		return beginCredentialsWireResult{}, invalidResponseDetail("begin_steamid_range")
	}
	if seen&(uint64(1)<<7) != 0 && !validAgreementURL(string(agreementURL)) {
		return beginCredentialsWireResult{}, invalidResponseDetail("begin_agreement_url")
	}
	result.requestID = append([]byte(nil), result.requestID...)
	result.agreementURL = string(agreementURL)
	result.serverMessage = string(serverMessage)
	return result, nil
}

func unmarshalAllowedChallenge(data []byte) (AllowedChallenge, *Error) {
	var result AllowedChallenge
	var associatedMessage []byte
	var seen uint64
	decoder := protobufDecoder{data: data}
	for {
		field, ok := decoder.next()
		if !ok {
			break
		}
		switch field.number {
		case 1:
			value, valid := fieldVarint(field)
			if !valid || value > math.MaxUint32 || !validChallengeType(ChallengeType(value)) || !markSingleton(&seen, field.number) {
				return AllowedChallenge{}, invalidResponseDetail("challenge_type_field")
			}
			result.Type = ChallengeType(value)
		case 2:
			value, valid := fieldBytes(field, 256)
			if !valid || !validProtocolText(value, 256, true) || !markSingleton(&seen, field.number) {
				return AllowedChallenge{}, invalidResponseDetail("challenge_message_field")
			}
			associatedMessage = value
		default:
			return AllowedChallenge{}, invalidResponseDetail("challenge_unknown_field")
		}
	}
	if !decoder.validEnd() {
		return AllowedChallenge{}, invalidResponseDetail("challenge_trailing_bytes")
	}
	if !validChallengeType(result.Type) {
		return AllowedChallenge{}, invalidResponseDetail("challenge_missing_type")
	}
	result.AssociatedMessage = string(associatedMessage)
	return result, nil
}

func unmarshalSteamGuardCodeResponse(data []byte) (string, *Error) {
	var agreementURL []byte
	var seen uint64
	decoder := protobufDecoder{data: data}
	for {
		field, ok := decoder.next()
		if !ok {
			break
		}
		// Steam adds fields to this response without notice. Rejecting unknown
		// numbers turned an accepted Steam Guard code into an invalid response,
		// so they are skipped; the agreement URL stays validated.
		if field.number != 7 {
			continue
		}
		if !markSingleton(&seen, field.number) {
			return "", invalidResponseDetail("guard_code_duplicate_agreement_field")
		}
		value, valid := fieldBytes(field, maxAgreementURLBytes)
		if !valid {
			return "", invalidResponseDetail("guard_code_agreement_field_bytes")
		}
		agreementURL = value
	}
	if !decoder.validEnd() {
		return "", invalidResponseDetail("guard_code_trailing_bytes")
	}
	// Steam sends this field explicitly even when no agreement is pending, and it
	// may carry a form this client would not open. Neither says anything about the
	// code that was just submitted, so neither is allowed to fail the submission:
	// an unusable value simply means there is no agreement URL to hand back.
	if !validAgreementURL(string(agreementURL)) {
		return "", nil
	}
	return string(agreementURL), nil
}

func unmarshalPollResponse(data []byte) (pollWireResult, *Error) {
	var result pollWireResult
	var challengeURL, refreshToken, accessToken, accountName, guardData, agreementURL []byte
	var seen uint64
	decoder := protobufDecoder{data: data}
	for {
		field, ok := decoder.next()
		if !ok {
			break
		}
		if !markSingleton(&seen, field.number) {
			return pollWireResult{}, invalidResponseDetail("poll_repeated_field")
		}
		switch field.number {
		case 1:
			value, valid := fieldVarint(field)
			if !valid || value == 0 {
				return pollWireResult{}, invalidResponseDetail("poll_client_id_field")
			}
			result.newClientID = value
		case 2:
			value, valid := fieldBytes(field, maxChallengeURLBytes)
			if !valid {
				return pollWireResult{}, invalidResponseDetail("poll_challenge_url_field")
			}
			challengeURL = value
		case 3:
			value, valid := fieldBytes(field, maxTokenBytes)
			if !valid || !validOpaqueBytes(value, maxTokenBytes) {
				return pollWireResult{}, invalidResponseDetail("poll_refresh_token_field")
			}
			refreshToken = value
		case 4:
			value, valid := fieldBytes(field, maxTokenBytes)
			if !valid || !validOpaqueBytes(value, maxTokenBytes) {
				return pollWireResult{}, invalidResponseDetail("poll_access_token_field")
			}
			accessToken = value
		case 5:
			value, valid := fieldVarint(field)
			if !valid || value > 1 {
				return pollWireResult{}, invalidResponseDetail("poll_interaction_field")
			}
			result.hadRemoteInteraction = value == 1
		case 6:
			value, valid := fieldBytes(field, maxAccountNameBytes)
			if !valid || !validProtocolText(value, maxAccountNameBytes, false) {
				return pollWireResult{}, invalidResponseDetail("poll_account_name_field")
			}
			accountName = value
		case 7:
			value, valid := fieldBytes(field, maxGuardDataBytes)
			if !valid || !validOpaqueBytes(value, maxGuardDataBytes) {
				return pollWireResult{}, invalidResponseDetail("poll_guard_data_field")
			}
			guardData = value
		case 8:
			value, valid := fieldBytes(field, maxAgreementURLBytes)
			if !valid {
				return pollWireResult{}, invalidResponseDetail("poll_agreement_field")
			}
			agreementURL = value
		default:
			return pollWireResult{}, invalidResponseDetail("poll_unknown_field")
		}
	}
	if !decoder.validEnd() {
		return pollWireResult{}, invalidResponseDetail("poll_trailing_bytes")
	}
	if seen&(uint64(1)<<2) != 0 && (result.newClientID == 0 || !validChallengeURL(string(challengeURL))) {
		return pollWireResult{}, invalidResponseDetail("poll_challenge_url")
	}
	if len(accountName) != 0 && !validProtocolString(string(accountName), maxAccountNameBytes, false) {
		return pollWireResult{}, invalidResponseDetail("poll_account_name")
	}
	if seen&(uint64(1)<<8) != 0 && !validAgreementURL(string(agreementURL)) {
		return pollWireResult{}, invalidResponseDetail("poll_agreement_url")
	}
	result.challengeURL = string(challengeURL)
	result.refreshToken = string(refreshToken)
	result.accessToken = string(accessToken)
	result.accountName = string(accountName)
	result.guardData = string(guardData)
	result.agreementURL = string(agreementURL)
	return result, nil
}

func unmarshalGenerateAccessTokenResponse(data []byte) (TokenResult, *Error) {
	result := TokenResult{State: AuthResultTokenIssued}
	var accessToken, refreshToken []byte
	var seen uint64
	decoder := protobufDecoder{data: data}
	for {
		field, ok := decoder.next()
		if !ok {
			break
		}
		// Steam adds fields to this response without notice. Rejecting unknown
		// numbers turned a perfectly good token grant into an invalid response,
		// so they are skipped; the two fields that matter stay validated.
		if field.number != 1 && field.number != 2 {
			continue
		}
		if !markSingleton(&seen, field.number) {
			return TokenResult{}, invalidResponse()
		}
		value, valid := fieldBytes(field, maxTokenBytes)
		if !valid || !validOpaqueBytes(value, maxTokenBytes) {
			return TokenResult{}, invalidResponse()
		}
		if field.number == 1 {
			accessToken = value
		} else {
			refreshToken = value
		}
	}
	if !decoder.validEnd() || len(accessToken) == 0 {
		return TokenResult{}, invalidResponse()
	}
	result.AccessToken = string(accessToken)
	result.RefreshToken = string(refreshToken)
	return result, nil
}

func unmarshalGetAuthSessionInfoResponse(data []byte) (AuthSessionInfo, *Error) {
	result := AuthSessionInfo{RequestedPersistence: PersistenceInvalid}
	var ipAddress, geoLocation, city, state, country, deviceName []byte
	var seen uint64
	decoder := protobufDecoder{data: data}
	for {
		field, ok := decoder.next()
		if !ok {
			break
		}
		if !markSingleton(&seen, field.number) {
			return AuthSessionInfo{}, invalidResponse()
		}
		switch field.number {
		case 1:
			value, valid := fieldBytes(field, maxIPAddressBytes)
			if !valid || !validProtocolText(value, maxIPAddressBytes, true) {
				return AuthSessionInfo{}, invalidResponse()
			}
			ipAddress = value
		case 2:
			value, valid := fieldBytes(field, maxGeoLocationBytes)
			if !valid || !validProtocolText(value, maxGeoLocationBytes, true) {
				return AuthSessionInfo{}, invalidResponse()
			}
			geoLocation = value
		case 3:
			value, valid := fieldBytes(field, maxLocationNameBytes)
			if !valid || !validProtocolText(value, maxLocationNameBytes, true) {
				return AuthSessionInfo{}, invalidResponse()
			}
			city = value
		case 4:
			value, valid := fieldBytes(field, maxLocationNameBytes)
			if !valid || !validProtocolText(value, maxLocationNameBytes, true) {
				return AuthSessionInfo{}, invalidResponse()
			}
			state = value
		case 5:
			value, valid := fieldBytes(field, maxCountryBytes)
			if !valid || !validProtocolText(value, maxCountryBytes, true) {
				return AuthSessionInfo{}, invalidResponse()
			}
			country = value
		case 6:
			value, valid := fieldVarint(field)
			if !valid || value > math.MaxUint32 || !validPlatform(PlatformType(value)) {
				return AuthSessionInfo{}, invalidResponse()
			}
			result.Platform = PlatformType(value)
		case 7:
			value, valid := fieldBytes(field, maxFriendlyNameBytes)
			if !valid || !validProtocolText(value, maxFriendlyNameBytes, false) {
				return AuthSessionInfo{}, invalidResponse()
			}
			deviceName = value
		case 8:
			value, valid := fieldInt32(field)
			if !valid || !validAuthVersion(value) {
				return AuthSessionInfo{}, invalidResponse()
			}
			result.Version = value
		case 9:
			value, valid := fieldVarint(field)
			if !valid || value > math.MaxUint32 || !validSecurityHistory(SecurityHistory(value)) {
				return AuthSessionInfo{}, invalidResponse()
			}
			result.LoginHistory = SecurityHistory(value)
		case 10:
			value, valid := fieldVarint(field)
			if !valid || value > 1 {
				return AuthSessionInfo{}, invalidResponse()
			}
			result.RequestorLocationMismatch = value == 1
		case 11:
			value, valid := fieldVarint(field)
			if !valid || value > 1 {
				return AuthSessionInfo{}, invalidResponse()
			}
			result.HighUsageLogin = value == 1
		case 12:
			value, valid := fieldInt32(field)
			if !valid || !validReportedPersistence(Persistence(value)) {
				return AuthSessionInfo{}, invalidResponse()
			}
			result.RequestedPersistence = Persistence(value)
		case 13:
			value, valid := fieldInt32(field)
			if !valid {
				return AuthSessionInfo{}, invalidResponse()
			}
			result.DeviceTrust = value
		case 14:
			value, valid := fieldVarint(field)
			if !valid || value > uint64(AppTypeSteamChat) {
				return AuthSessionInfo{}, invalidResponse()
			}
			result.App = AppType(value)
		default:
			return AuthSessionInfo{}, invalidResponse()
		}
	}
	if !decoder.validEnd() || !validIPAddress(string(ipAddress)) ||
		!validProtocolString(string(geoLocation), maxGeoLocationBytes, true) ||
		!validProtocolString(string(city), maxLocationNameBytes, true) ||
		!validProtocolString(string(state), maxLocationNameBytes, true) ||
		!validProtocolString(string(country), maxCountryBytes, true) ||
		!validProtocolString(string(deviceName), maxFriendlyNameBytes, false) ||
		result.Platform == 0 || result.Version == 0 {
		return AuthSessionInfo{}, invalidResponse()
	}
	result.IPAddress = string(ipAddress)
	result.GeoLocation = string(geoLocation)
	result.City = string(city)
	result.State = string(state)
	result.Country = string(country)
	result.DeviceFriendlyName = string(deviceName)
	return result, nil
}

func fieldInt32(field protobufField) (int32, bool) {
	value, valid := fieldVarint(field)
	if !valid {
		return 0, false
	}
	converted := int32(value)
	if converted >= 0 {
		return converted, value == uint64(converted)
	}
	return converted, value == uint64(int64(converted))
}

func boolVarint(value bool) uint64 {
	if value {
		return 1
	}
	return 0
}

func invalidResponse() *Error {
	return protocolError(CodeInvalidResponse, StateFailed)
}

// invalidResponseDetail labels which check rejected the response. detail is a
// fixed identifier chosen at the call site, never any part of the response.
func invalidResponseDetail(detail string) *Error {
	err := protocolError(CodeInvalidResponse, StateFailed)
	err.Detail = detail
	return err
}
