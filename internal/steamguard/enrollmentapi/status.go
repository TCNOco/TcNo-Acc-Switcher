package enrollmentapi

import (
	"context"
	"math"
	"time"
)

const queryStatusEndpoint = "https://api.steampowered.com/ITwoFactorService/QueryStatus/v1"

// Highest field this build reads from the status response. Steam defines more;
// they are skipped like any other unknown field.
const maxKnownStatusResponseField = 6

// StatusResult is what Steam currently holds on the account. TokenGID
// identifies the active authenticator. It is an opaque identifier rather than a
// secret, which is what makes it usable as proof that a stored authenticator is
// the live one.
type StatusResult struct {
	AuthenticatorType uint32
	TokenGID          string
}

func marshalStatusRequest(steamID uint64) []byte {
	return appendFixed64(nil, 1, steamID)
}

func unmarshalStatusResponse(data []byte) (StatusResult, error) {
	var result StatusResult
	var seen uint64
	decoder := wireDecoder{data: data}
	for {
		field, ok := decoder.next()
		if !ok {
			break
		}
		if field.number > maxKnownStatusResponseField {
			continue
		}
		if !markSeen(&seen, field.number) {
			return StatusResult{}, invalidResponse("duplicate field")
		}
		switch field.number {
		case 3:
			if field.typeID != wireVarint || field.varint > math.MaxUint32 {
				return StatusResult{}, invalidResponse("authenticator type")
			}
			result.AuthenticatorType = uint32(field.varint)
		case 6:
			if field.typeID != wireBytes || !validText(field.bytes, maxTokenGIDBytes, true) {
				return StatusResult{}, invalidResponse("token GID")
			}
			result.TokenGID = string(field.bytes)
		default:
			continue
		}
	}
	if !decoder.validEnd() {
		return StatusResult{}, invalidResponse("trailing bytes")
	}
	return result, nil
}

// QueryStatus reads the account's current two-factor state. It is a read: it
// changes nothing on Steam and needs no confirmation code, so it is safe to
// call after a finalize has already been refused.
func (c *Client) QueryStatus(ctx context.Context, steamID uint64, accessToken []byte, timeout time.Duration) (StatusResult, error) {
	if c == nil || c.protocol == nil || ctx == nil || timeout <= 0 || timeout > requestTimeout ||
		!validSteamID(steamID) || !validToken(accessToken) {
		return StatusResult{}, ErrInvalidRequest
	}
	message := marshalStatusRequest(steamID)
	response, callErr := c.call(ctx, queryStatusEndpoint, accessToken, message, timeout)
	wipe(message)
	if callErr != nil {
		return StatusResult{}, callErr
	}
	defer wipe(response)
	return unmarshalStatusResponse(response)
}
