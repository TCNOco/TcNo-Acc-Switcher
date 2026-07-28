package protocol

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"math"
	"math/big"
	"runtime"
	"time"
	"unicode/utf8"
)

const (
	getPasswordRSAKeyEndpoint = "https://api.steampowered.com/IAuthenticationService/GetPasswordRSAPublicKey/v1"
	minPasswordRSAKeyBits     = 2048
	maxPasswordRSAKeyBits     = 4096
	maxPasswordBytes          = 1024
	maxRSAExponentHexBytes    = 8
)

// PasswordCredentialsRequest contains the non-secret metadata used to start a
// password authentication session. The plaintext password is deliberately not
// part of this reusable DTO.
type PasswordCredentialsRequest struct {
	DeviceFriendlyName string
	AccountName        string
	RememberLogin      bool
	Platform           PlatformType
	Persistence        Persistence
	WebsiteID          string
	Device             DeviceDetails
	GuardData          string
	Language           uint32
	QoSLevel           int32
}

type passwordRSAKey struct {
	publicKey rsa.PublicKey
	timestamp uint64
}

// BeginAuthSessionWithPassword fetches Steam's current account key, encrypts
// password with RSA PKCS#1 v1.5, and starts a credentials session. password is
// borrowed only for this call; the caller retains ownership and should wipe it
// when no longer needed.
func (a *AuthenticationClient) BeginAuthSessionWithPassword(
	ctx context.Context,
	request PasswordCredentialsRequest,
	password []byte,
	timeout time.Duration,
) (BeginCredentialsResult, error) {
	if a == nil || a.client == nil || a.entropy == nil || ctx == nil ||
		timeout <= 0 || timeout > MaxRequestTimeout || !validPlaintextPassword(password) {
		return BeginCredentialsResult{}, protocolError(CodeInvalidRequest, StateInvalid)
	}
	if validationErr := validatePasswordCredentialsRequest(request); validationErr != nil {
		return BeginCredentialsResult{}, validationErr
	}

	operationCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	key, err := a.getPasswordRSAPublicKey(operationCtx, request.AccountName, timeout)
	if err != nil {
		return BeginCredentialsResult{}, err
	}
	encryptedPassword, encryptErr := encryptPasswordPKCS1v15(key.publicKey, password)
	runtime.KeepAlive(password)
	if encryptErr != nil {
		return BeginCredentialsResult{}, encryptErr
	}

	beginRequest := BeginCredentialsRequest{
		DeviceFriendlyName:  request.DeviceFriendlyName,
		AccountName:         request.AccountName,
		EncryptedPassword:   encryptedPassword,
		EncryptionTimestamp: key.timestamp,
		RememberLogin:       request.RememberLogin,
		Platform:            request.Platform,
		Persistence:         request.Persistence,
		WebsiteID:           request.WebsiteID,
		Device:              request.Device,
		GuardData:           request.GuardData,
		Language:            request.Language,
		QoSLevel:            request.QoSLevel,
	}
	return a.BeginAuthSessionViaCredentials(operationCtx, beginRequest, timeout)
}

func (a *AuthenticationClient) getPasswordRSAPublicKey(ctx context.Context, accountName string, timeout time.Duration) (passwordRSAKey, error) {
	if a == nil || a.client == nil || ctx == nil ||
		!validProtocolString(accountName, maxAccountNameBytes, false) {
		return passwordRSAKey{}, protocolError(CodeInvalidRequest, StateInvalid)
	}
	message := marshalGetPasswordRSAPublicKeyRequest(accountName)
	// Read-only service method: GET only, so this cannot use callResponse.
	response, err := a.getResponse(ctx, getPasswordRSAKeyEndpoint, message, timeout)
	wipeBytes(message)
	if err != nil {
		return passwordRSAKey{}, err
	}
	defer wipeBytes(response.Body)
	if resultErr := requireSuccessfulEResult(response); resultErr != nil {
		return passwordRSAKey{}, resultErr
	}
	key, parseErr := unmarshalGetPasswordRSAPublicKeyResponse(response.Body)
	if parseErr != nil {
		return passwordRSAKey{}, parseErr
	}
	return key, nil
}

func marshalGetPasswordRSAPublicKeyRequest(accountName string) []byte {
	return appendStringField(nil, 1, accountName)
}

func unmarshalGetPasswordRSAPublicKeyResponse(data []byte) (passwordRSAKey, *Error) {
	var modulusHex, exponentHex []byte
	var timestamp uint64
	var seen uint64
	decoder := protobufDecoder{data: data}
	for {
		field, ok := decoder.next()
		if !ok {
			break
		}
		if !markSingleton(&seen, field.number) {
			return passwordRSAKey{}, invalidResponse()
		}
		switch field.number {
		case 1:
			value, valid := fieldBytes(field, maxPasswordRSAKeyBits/4)
			if !valid {
				return passwordRSAKey{}, invalidResponse()
			}
			modulusHex = value
		case 2:
			value, valid := fieldBytes(field, maxRSAExponentHexBytes)
			if !valid {
				return passwordRSAKey{}, invalidResponse()
			}
			exponentHex = value
		case 3:
			value, valid := fieldVarint(field)
			if !valid || value == 0 {
				return passwordRSAKey{}, invalidResponse()
			}
			timestamp = value
		default:
			return passwordRSAKey{}, invalidResponse()
		}
	}
	const required = uint64(1)<<1 | uint64(1)<<2 | uint64(1)<<3
	if !decoder.validEnd() || seen != required || !validHexBytes(modulusHex) || !validHexBytes(exponentHex) {
		return passwordRSAKey{}, invalidResponse()
	}

	modulusBytes := make([]byte, hex.DecodedLen(len(modulusHex)))
	if _, err := hex.Decode(modulusBytes, modulusHex); err != nil {
		wipeBytes(modulusBytes)
		return passwordRSAKey{}, invalidResponse()
	}
	defer wipeBytes(modulusBytes)
	exponentBytes := make([]byte, hex.DecodedLen(len(exponentHex)))
	if _, err := hex.Decode(exponentBytes, exponentHex); err != nil {
		wipeBytes(exponentBytes)
		return passwordRSAKey{}, invalidResponse()
	}
	defer wipeBytes(exponentBytes)

	modulus := new(big.Int).SetBytes(modulusBytes)
	exponentValue := new(big.Int).SetBytes(exponentBytes)
	if modulus.BitLen() < minPasswordRSAKeyBits || modulus.BitLen() > maxPasswordRSAKeyBits || modulus.Bit(0) == 0 ||
		!exponentValue.IsInt64() || exponentValue.Sign() <= 0 || exponentValue.Int64() > math.MaxInt32 {
		return passwordRSAKey{}, invalidResponse()
	}
	exponent := int(exponentValue.Int64())
	if exponent < 3 || exponent&1 == 0 {
		return passwordRSAKey{}, invalidResponse()
	}
	return passwordRSAKey{
		publicKey: rsa.PublicKey{N: modulus, E: exponent},
		timestamp: timestamp,
	}, nil
}

func encryptPasswordPKCS1v15(publicKey rsa.PublicKey, password []byte) (string, *Error) {
	if publicKey.N == nil || publicKey.E < 3 ||
		len(password) > publicKey.Size()-11 {
		return "", protocolError(CodeInvalidRequest, StateInvalid)
	}
	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, &publicKey, password)
	if err != nil {
		wipeBytes(ciphertext)
		return "", protocolError(CodeEntropy, StateFailed)
	}
	defer wipeBytes(ciphertext)
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(ciphertext)))
	base64.StdEncoding.Encode(encoded, ciphertext)
	result := string(encoded)
	wipeBytes(encoded)
	return result, nil
}

func validatePasswordCredentialsRequest(request PasswordCredentialsRequest) *Error {
	placeholder := make([]byte, minPasswordRSAKeyBits/8)
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(placeholder)))
	base64.StdEncoding.Encode(encoded, placeholder)
	wipeBytes(placeholder)
	validationRequest := BeginCredentialsRequest{
		DeviceFriendlyName:  request.DeviceFriendlyName,
		AccountName:         request.AccountName,
		EncryptedPassword:   string(encoded),
		EncryptionTimestamp: 1,
		RememberLogin:       request.RememberLogin,
		Platform:            request.Platform,
		Persistence:         request.Persistence,
		WebsiteID:           request.WebsiteID,
		Device:              request.Device,
		GuardData:           request.GuardData,
		Language:            request.Language,
		QoSLevel:            request.QoSLevel,
	}
	wipeBytes(encoded)
	return validateBeginCredentialsRequest(validationRequest)
}

func validPlaintextPassword(password []byte) bool {
	return len(password) > 0 && len(password) <= maxPasswordBytes && utf8.Valid(password)
}

func validHexBytes(value []byte) bool {
	if len(value) == 0 || len(value)&1 != 0 {
		return false
	}
	for _, current := range value {
		if (current < '0' || current > '9') && (current < 'a' || current > 'f') && (current < 'A' || current > 'F') {
			return false
		}
	}
	return true
}
