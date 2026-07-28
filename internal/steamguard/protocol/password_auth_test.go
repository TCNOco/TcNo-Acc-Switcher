package protocol

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math/big"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testPasswordRSAModulus = "C239450AE331D4EF94B4DD7B18E43B9712B4ACFF595F423A4C2EC3862E526966A5205B7D650C4D216C9B8E9DEEC2512A6AB18EE97B10ADF44A5EDAD4DEC34003DD9890D704EB5B9DE6AD7CB45F70F912465993E911C7421307B29A1717D80DBD4A289D0351BF37F0907B0293BDADE69E5515D8E9F620806E5D33EDEC49F7EB8C6F279BA28AFC01FFB52B9DD36063AB92B44FF712FEA50BA8A04BF5140EAF2A1A20FB321D4230B5A42FB0CE62D62C163E12BE8AA95015DC3A6B3296F79D75A278ECB0D18083CF9199D3CC3FD6C1D4484D52360C1A83765D71920C7419F4A0CC712A9B3D5C93F68D2529DCB63A9C8AB3934957C75ACEDBD4C6914142474297FDD9"

const testPasswordRSAPrivateExponent = "B654B53027611C995D6CFD8F162B0C96228562F2C49FDCB885D450D1A2A2D337FD44871F0CC1A3970132778C641C1FBE4633320A95F16E9CAB44A902B5AD6E67329C8B3C8FEDB33064E1F0F413B526DDB5155AF9AE2AF528904D66C2CF2B909A6708017EA03B76F46B6E4F590AF43A4FE168851DFE653CAC5EEAE52CB1B400774DE1E79A8F537768657672963D78755F853E4FD221ABFE6CDECC34527C19C7A0129BE731C4609B8028108F1722A706E6C7BB12413702628A8DD0133AD0CD4D3A2E23FBDCD607410ECF32D706A3AF5F775C41837B9FA7A21C43C5FD338D5944741F56E0A1DBCDA7F5396FA73034CCA505B14E7ABEFBDCACB2B2AF6B4E27E225D1"

func TestBeginAuthSessionWithPasswordUsesExactKeyRequestAndRSAEncryption(t *testing.T) {
	t.Parallel()

	password := []byte("correct horse")
	var calls atomic.Int32
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch calls.Add(1) {
		case 1:
			assertReadOnlyAuthRequest(t, request, getPasswordRSAKeyEndpoint)
			message := decodeAuthQuery(t, request)
			if got := strings.ToUpper(hex.EncodeToString(message)); got != "0A0C6163636F756E745F6E616D65" {
				t.Fatalf("key request protobuf = %s", got)
			}
			body := passwordRSAResponse(testPasswordRSAModulus, "010001", 123456)
			return response(request, http.StatusOK, steamEResultHeader("1"), body), nil
		case 2:
			assertAuthRequest(t, request, beginCredentialsEndpoint)
			message := decodeAuthForm(t, request)
			if bytes.Contains(message, password) {
				t.Fatal("credentials protobuf contains plaintext password")
			}
			fields := decodeFields(t, message)
			if string(fields[2].bytes) != "account_name" || fields[4].varint != 123456 {
				t.Fatalf("encrypted credentials fields are incorrect: %#v", fields)
			}
			assertPKCS1v15Ciphertext(t, string(fields[3].bytes), password)
			challenge := appendVarintField(nil, 1, uint64(ChallengeDeviceCode))
			body := appendVarintField(nil, 1, 4123)
			body = appendBytesField(body, 2, []byte{1, 2, 3, 4})
			body = appendFloat32Field(body, 3, 5)
			body = appendBytesField(body, 4, challenge)
			body = appendVarintField(body, 5, testSteamID)
			return response(request, http.StatusOK, nil, body), nil
		default:
			t.Fatal("unexpected transport call")
			return nil, errors.New("unexpected transport call")
		}
	})
	auth := newAuthenticationClientForTest(NewClient(Options{Transport: transport}), constantReader(0x42))
	result, err := auth.BeginAuthSessionWithPassword(context.Background(), validPasswordRequest(), password, 3*time.Second)
	if err != nil {
		t.Fatalf("begin error = %#v", err)
	}
	if result.State != AuthResultChallengeRequired || result.Session.ClientID() != 4123 || result.Session.SteamID() != testSteamID {
		t.Fatalf("begin result = %#v", result)
	}
	if calls.Load() != 2 {
		t.Fatalf("transport calls = %d", calls.Load())
	}
	if string(password) != "correct horse" {
		t.Fatal("borrowed caller password was modified")
	}
}

func TestPasswordKeyResponseStrictValidation(t *testing.T) {
	t.Parallel()

	valid := passwordRSAResponse(testPasswordRSAModulus, "010001", 123456)
	key, err := unmarshalGetPasswordRSAPublicKeyResponse(valid)
	if err != nil {
		t.Fatal(err)
	}
	if key.publicKey.N.BitLen() != 2048 || key.publicKey.E != 65537 || key.timestamp != 123456 {
		t.Fatalf("parsed key = bits:%d exponent:%d timestamp:%d", key.publicKey.N.BitLen(), key.publicKey.E, key.timestamp)
	}

	tests := map[string][]byte{
		"missing timestamp": appendStringField(appendStringField(nil, 1, testPasswordRSAModulus), 2, "010001"),
		"duplicate modulus": appendStringField(valid, 1, testPasswordRSAModulus),
		"unknown field":     appendVarintField(valid, 4, 1),
		"short modulus":     passwordRSAResponse(strings.Repeat("A1", 128), "010001", 123456),
		"even modulus":      passwordRSAResponse(testPasswordRSAModulus[:len(testPasswordRSAModulus)-1]+"0", "010001", 123456),
		"even exponent":     passwordRSAResponse(testPasswordRSAModulus, "02", 123456),
		"invalid hex":       passwordRSAResponse("Z"+testPasswordRSAModulus[1:], "010001", 123456),
		"zero timestamp":    passwordRSAResponse(testPasswordRSAModulus, "010001", 0),
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			if _, parseErr := unmarshalGetPasswordRSAPublicKeyResponse(body); parseErr == nil || parseErr.Code != CodeInvalidResponse {
				t.Fatalf("parse error = %#v", parseErr)
			}
		})
	}
}

func TestPasswordAuthenticationRequiresSuccessfulKeyEResult(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := passwordRSAResponse(testPasswordRSAModulus, "010001", 123456)
		return response(request, http.StatusOK, steamEResultHeader("15"), body), nil
	})
	auth := newAuthenticationClientForTest(NewClient(Options{Transport: transport}), constantReader(0x42))
	password := []byte("do-not-leak")
	_, err := auth.BeginAuthSessionWithPassword(context.Background(), validPasswordRequest(), password, time.Second)
	protocolErr := assertProtocolCode(t, err, CodeSteamResult)
	if !protocolErr.HasEResult || protocolErr.EResult != 15 || strings.Contains(err.Error(), string(password)) {
		t.Fatalf("key error = %#v", err)
	}
}

func TestPasswordAuthenticationRejectsInputBeforeTransport(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("must not run")
	})
	auth := newAuthenticationClientForTest(NewClient(Options{Transport: transport}), constantReader(0x42))

	tests := []struct {
		name     string
		request  PasswordCredentialsRequest
		password []byte
	}{
		{name: "empty password", request: validPasswordRequest()},
		{name: "invalid utf8", request: validPasswordRequest(), password: []byte{0xff}},
		{name: "invalid account", request: func() PasswordCredentialsRequest {
			request := validPasswordRequest()
			request.AccountName = " account"
			return request
		}(), password: []byte("secret")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := auth.BeginAuthSessionWithPassword(context.Background(), test.request, test.password, time.Second)
			assertProtocolCode(t, err, CodeInvalidRequest)
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("transport calls = %d", calls.Load())
	}
}

func passwordRSAResponse(modulus, exponent string, timestamp uint64) []byte {
	body := appendStringField(nil, 1, modulus)
	body = appendStringField(body, 2, exponent)
	return appendVarintField(body, 3, timestamp)
}

func assertPKCS1v15Ciphertext(t *testing.T, encoded string, want []byte) {
	t.Helper()
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(ciphertext) != minPasswordRSAKeyBits/8 {
		t.Fatal("encrypted password is not a 2048-bit RSA ciphertext")
	}
	modulus, ok := new(big.Int).SetString(testPasswordRSAModulus, 16)
	if !ok {
		t.Fatal("invalid test modulus")
	}
	privateExponent, ok := new(big.Int).SetString(testPasswordRSAPrivateExponent, 16)
	if !ok {
		t.Fatal("invalid test private exponent")
	}
	encodedMessage := new(big.Int).Exp(new(big.Int).SetBytes(ciphertext), privateExponent, modulus).FillBytes(make([]byte, len(ciphertext)))
	if encodedMessage[0] != 0 || encodedMessage[1] != 2 {
		t.Fatal("encrypted password does not use PKCS#1 v1.5 block type 2")
	}
	separator := bytes.IndexByte(encodedMessage[2:], 0)
	if separator < 8 {
		t.Fatal("encrypted password has invalid PKCS#1 v1.5 padding")
	}
	separator += 2
	if !bytes.Equal(encodedMessage[separator+1:], want) {
		t.Fatal("RSA ciphertext does not contain the supplied password")
	}
}

func validPasswordRequest() PasswordCredentialsRequest {
	request := validBeginRequest()
	return PasswordCredentialsRequest{
		DeviceFriendlyName: request.DeviceFriendlyName,
		AccountName:        request.AccountName,
		RememberLogin:      request.RememberLogin,
		Platform:           request.Platform,
		Persistence:        request.Persistence,
		WebsiteID:          request.WebsiteID,
		Device:             request.Device,
		GuardData:          request.GuardData,
		Language:           request.Language,
		QoSLevel:           request.QoSLevel,
	}
}

type constantReader byte

func (r constantReader) Read(destination []byte) (int, error) {
	for index := range destination {
		destination[index] = byte(r)
	}
	return len(destination), nil
}
