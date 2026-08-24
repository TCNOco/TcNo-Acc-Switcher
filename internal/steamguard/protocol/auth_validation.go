package protocol

import (
	"encoding/base64"
	"math"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxFriendlyNameBytes = 128
	maxAccountNameBytes  = 64
	maxWebsiteIDBytes    = 64
	maxGuardDataBytes    = 4096
	maxTokenBytes        = 8192
	maxRequestIDBytes    = 128
	maxMessageBytes      = 512
	maxAgreementURLBytes = 2048
	maxChallengeURLBytes = 256
	maxIPAddressBytes    = 64
	maxGeoLocationBytes  = 64
	maxLocationNameBytes = 128
	maxCountryBytes      = 8
	localSessionIDBytes  = 16
	steamIDAccountMin    = uint64(76561197960265728)
	steamIDAccountMax    = steamIDAccountMin + uint64(^uint32(0))
)

func validateBeginCredentialsRequest(request BeginCredentialsRequest) *Error {
	if !validProtocolString(request.DeviceFriendlyName, maxFriendlyNameBytes, false) ||
		!validProtocolString(request.AccountName, maxAccountNameBytes, false) ||
		!validProtocolString(request.WebsiteID, maxWebsiteIDBytes, false) ||
		request.EncryptionTimestamp == 0 ||
		!validPlatform(request.Platform) ||
		!validPersistence(request.Persistence) ||
		request.QoSLevel < 0 || request.QoSLevel > 2 || request.Language > 999 {
		return protocolError(CodeInvalidRequest, StateInvalid)
	}
	if request.GuardData != "" && !validOpaqueString(request.GuardData, maxGuardDataBytes) {
		return protocolError(CodeInvalidRequest, StateInvalid)
	}
	decodedPassword, err := base64.StdEncoding.DecodeString(request.EncryptedPassword)
	if err != nil || len(decodedPassword) < 128 || len(decodedPassword) > 512 {
		wipeBytes(decodedPassword)
		return protocolError(CodeInvalidRequest, StateInvalid)
	}
	wipeBytes(decodedPassword)
	if !validDeviceDetails(request.Device) || request.Device.Platform != request.Platform || request.Device.FriendlyName != request.DeviceFriendlyName {
		return protocolError(CodeInvalidRequest, StateInvalid)
	}
	return nil
}

func validDeviceDetails(device DeviceDetails) bool {
	return validProtocolString(device.FriendlyName, maxFriendlyNameBytes, false) &&
		validPlatform(device.Platform) &&
		device.OSType >= 0 && device.OSType <= 1000 &&
		device.ClientCount <= 64 &&
		len(device.MachineID) <= 256 &&
		device.App <= AppTypeSteamChat
}

func validPlatform(platform PlatformType) bool {
	return platform >= PlatformSteamClient && platform <= PlatformMobileApp
}

func validPersistence(persistence Persistence) bool {
	return persistence == PersistenceEphemeral || persistence == PersistencePersistent
}

func validReportedPersistence(persistence Persistence) bool {
	return persistence >= PersistenceInvalid && persistence <= PersistencePersistent
}

func validAuthVersion(version int32) bool {
	return version > 0 && version <= math.MaxUint16
}

func validSecurityHistory(history SecurityHistory) bool {
	return history <= SecurityHistoryNoPriorHistory
}

func validIPAddress(value string) bool {
	if value == "" {
		return true
	}
	if !validProtocolString(value, maxIPAddressBytes, false) {
		return false
	}
	_, err := netip.ParseAddr(value)
	return err == nil
}

func validChallengeType(challenge ChallengeType) bool {
	return challenge >= ChallengeNone && challenge <= ChallengeLegacyMachineAuth
}

func validCodeChallenge(challenge ChallengeType) bool {
	return challenge == ChallengeEmailCode || challenge == ChallengeDeviceCode
}

func validSteamGuardCode(code string) bool {
	if len(code) != 5 {
		return false
	}
	for i := range len(code) {
		value := code[i]
		if (value < '0' || value > '9') && (value < 'A' || value > 'Z') {
			return false
		}
	}
	return true
}

func validateAuthSession(session AuthSession) bool {
	decodedID, err := base64.RawURLEncoding.DecodeString(session.id)
	validID := err == nil && len(decodedID) == localSessionIDBytes
	wipeBytes(decodedID)
	return validID && session.clientID != 0 && validAccountSteamID(session.steamID) &&
		len(session.requestID) > 0 && len(session.requestID) <= maxRequestIDBytes && hasNonzeroByte(session.requestID)
}

// validatePollableSession is validateAuthSession with the SteamID requirement
// lifted for a QR session, which has none until it is scanned. Polling is the
// only call that takes one: a guard code is submitted for a named account, and
// marshalSteamGuardCodeRequest would put a zero SteamID on the wire.
func validatePollableSession(session AuthSession) bool {
	if session.viaQR && session.steamID == 0 {
		return validateAuthSessionExceptSteamID(session)
	}
	return validateAuthSession(session)
}

func validateAuthSessionExceptSteamID(session AuthSession) bool {
	decodedID, err := base64.RawURLEncoding.DecodeString(session.id)
	validID := err == nil && len(decodedID) == localSessionIDBytes
	wipeBytes(decodedID)
	return validID && session.clientID != 0 &&
		len(session.requestID) > 0 && len(session.requestID) <= maxRequestIDBytes && hasNonzeroByte(session.requestID)
}

func validateBeginQRRequest(request BeginQRRequest) *Error {
	if !validProtocolString(request.DeviceFriendlyName, maxFriendlyNameBytes, false) ||
		!validProtocolString(request.WebsiteID, maxWebsiteIDBytes, false) ||
		!validPlatform(request.Platform) {
		return protocolError(CodeInvalidRequest, StateInvalid)
	}
	if !validDeviceDetails(request.Device) || request.Device.Platform != request.Platform ||
		request.Device.FriendlyName != request.DeviceFriendlyName {
		return protocolError(CodeInvalidRequest, StateInvalid)
	}
	return nil
}

// challengeURLNamesClient checks that a challenge URL points at the session it
// arrived with. validChallengeURL already fixes the shape; this is what stops a
// response naming one session and handing back a URL that signs into another.
func challengeURLNamesClient(value string, clientID uint64) bool {
	return validChallengeURL(value) && strings.HasSuffix(value, "/"+strconv.FormatUint(clientID, 10))
}

func validAccountSteamID(steamID uint64) bool {
	return steamID >= steamIDAccountMin && steamID <= steamIDAccountMax
}

func hasNonzeroByte(value []byte) bool {
	for _, current := range value {
		if current != 0 {
			return true
		}
	}
	return false
}

func validProtocolText(value []byte, max int, allowEmpty bool) bool {
	if len(value) > max || (!allowEmpty && len(value) == 0) || !utf8.Valid(value) {
		return false
	}
	for _, character := range string(value) {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func validProtocolString(value string, max int, allowEmpty bool) bool {
	return validProtocolText([]byte(value), max, allowEmpty) && value == strings.TrimSpace(value)
}

func validOpaqueString(value string, max int) bool {
	return validOpaqueBytes([]byte(value), max)
}

func validOpaqueBytes(value []byte, max int) bool {
	if len(value) == 0 || len(value) > max {
		return false
	}
	for index := range len(value) {
		if value[index] < 0x21 || value[index] > 0x7e {
			return false
		}
	}
	return true
}

func validAgreementURL(value string) bool {
	if len(value) == 0 || len(value) > maxAgreementURLBytes {
		return false
	}
	_, err := validateEndpoint(value, RouteTransfer)
	return err == nil
}

func validChallengeURL(value string) bool {
	if len(value) == 0 || len(value) > maxChallengeURLBytes {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "s.team" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if parsed.EscapedPath() != parsed.Path {
		return false
	}
	parts := strings.Split(parsed.Path, "/")
	if len(parts) != 4 || parts[0] != "" || parts[1] != "q" || parts[2] != "1" {
		return false
	}
	clientID, parseErr := strconv.ParseUint(parts[3], 10, 64)
	return parseErr == nil && clientID != 0 && strconv.FormatUint(clientID, 10) == parts[3]
}

func wipeBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
