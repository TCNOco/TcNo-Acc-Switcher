package enrollmentapi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

const (
	wireVarint  = 0
	wireFixed64 = 1
	wireBytes   = 2
)

// Highest field number each response is known to define. Anything above is a
// field Steam added later; protobuf exists to let a reader skip those, and
// rejecting them would turn a routine server-side addition into a hard failure
// to enrol.
const (
	maxKnownAddResponseField      = 12
	maxKnownFinalizeResponseField = 4
)

// invalidResponse names which check rejected the response. The reason carries
// no field contents, so it is safe to log next to an enrolment failure.
func invalidResponse(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidResponse, reason)
}

type wireField struct {
	number  uint32
	typeID  uint8
	varint  uint64
	fixed64 uint64
	bytes   []byte
}

type wireDecoder struct {
	data   []byte
	offset int
}

func (d *wireDecoder) next() (wireField, bool) {
	if d.offset == len(d.data) {
		return wireField{}, false
	}
	tag, size, ok := consumeVarint(d.data[d.offset:])
	if !ok {
		d.offset = -1
		return wireField{}, false
	}
	d.offset += size
	field := wireField{number: uint32(tag >> 3), typeID: uint8(tag & 7)}
	if field.number == 0 || field.number >= 64 {
		d.offset = -1
		return wireField{}, false
	}
	switch field.typeID {
	case wireVarint:
		field.varint, size, ok = consumeVarint(d.data[d.offset:])
		if !ok {
			d.offset = -1
			return wireField{}, false
		}
		d.offset += size
	case wireFixed64:
		if len(d.data)-d.offset < 8 {
			d.offset = -1
			return wireField{}, false
		}
		field.fixed64 = binary.LittleEndian.Uint64(d.data[d.offset : d.offset+8])
		d.offset += 8
	case wireBytes:
		length, consumed, valid := consumeVarint(d.data[d.offset:])
		if !valid || length > uint64(len(d.data)-d.offset-consumed) {
			d.offset = -1
			return wireField{}, false
		}
		d.offset += consumed
		field.bytes = d.data[d.offset : d.offset+int(length)]
		d.offset += int(length)
	default:
		d.offset = -1
		return wireField{}, false
	}
	return field, true
}

func (d *wireDecoder) validEnd() bool { return d.offset == len(d.data) }

func consumeVarint(data []byte) (uint64, int, bool) {
	value, size := binary.Uvarint(data)
	if size <= 0 || size != varintSize(value) {
		return 0, 0, false
	}
	return value, size, true
}

func varintSize(value uint64) int {
	size := 1
	for value >= 1<<7 {
		value >>= 7
		size++
	}
	return size
}

func appendTag(dst []byte, number uint32, typeID uint8) []byte {
	return binary.AppendUvarint(dst, uint64(number)<<3|uint64(typeID))
}

func appendVarint(dst []byte, number uint32, value uint64) []byte {
	dst = appendTag(dst, number, wireVarint)
	return binary.AppendUvarint(dst, value)
}

func appendFixed64(dst []byte, number uint32, value uint64) []byte {
	dst = appendTag(dst, number, wireFixed64)
	return binary.LittleEndian.AppendUint64(dst, value)
}

func appendBytes(dst []byte, number uint32, value []byte) []byte {
	dst = appendTag(dst, number, wireBytes)
	dst = binary.AppendUvarint(dst, uint64(len(value)))
	return append(dst, value...)
}

func marshalAddRequest(steamID, authenticatorTime uint64, deviceID string) []byte {
	message := make([]byte, 0, 96)
	message = appendFixed64(message, 1, steamID)
	message = appendVarint(message, 2, authenticatorTime)
	message = appendVarint(message, 4, 1)
	message = appendBytes(message, 5, []byte(deviceID))
	return appendVarint(message, 8, 2)
}

// validateSMS sets validate_sms_code. It must reflect how Steam actually asked
// the user to confirm: telling Steam to validate an SMS code when the code was
// emailed makes it reject an otherwise correct finalize. Left unset for email
// and unknown confirmation types, which is the protobuf default of false.
func marshalFinalizeRequest(steamID uint64, authenticatorCode []byte, authenticatorTime uint64, confirmationCode []byte, validateSMS bool) []byte {
	message := make([]byte, 0, 80)
	message = appendFixed64(message, 1, steamID)
	message = appendBytes(message, 2, authenticatorCode)
	message = appendVarint(message, 3, authenticatorTime)
	message = appendBytes(message, 4, confirmationCode)
	if !validateSMS {
		return message
	}
	return appendVarint(message, 6, 1)
}

type addWireResult struct {
	pending *PendingEnrollment
	status  int32
}

func unmarshalAddResponse(data []byte) (result addWireResult, err error) {
	pending := &PendingEnrollment{}
	result.pending = pending
	valid := false
	defer func() {
		if !valid {
			pending.Destroy()
			result.pending = nil
		}
	}()
	var seen uint64
	var confirm uint64
	decoder := wireDecoder{data: data}
	for {
		field, ok := decoder.next()
		if !ok {
			break
		}
		if field.number > maxKnownAddResponseField {
			continue
		}
		if !markSeen(&seen, field.number) {
			return addWireResult{}, invalidResponse("duplicate field")
		}
		switch field.number {
		case 1:
			if field.typeID != wireBytes || len(field.bytes) > 64 {
				return addWireResult{}, invalidResponse("shared secret")
			}
			pending.SharedSecret = append([]byte(nil), field.bytes...)
		case 2:
			if field.typeID != wireFixed64 {
				return addWireResult{}, invalidResponse("serial number")
			}
			pending.SerialNumber = field.fixed64
		case 3:
			if field.typeID != wireBytes || !validText(field.bytes, 32, false) {
				return addWireResult{}, invalidResponse("revocation code")
			}
			pending.RevocationCode = append([]byte(nil), field.bytes...)
		case 4:
			if field.typeID != wireBytes || !validText(field.bytes, maxURIBytes, false) {
				return addWireResult{}, invalidResponse("otpauth URI")
			}
			pending.URI = append([]byte(nil), field.bytes...)
		case 5:
			if field.typeID != wireVarint {
				return addWireResult{}, invalidResponse("server time")
			}
			pending.ServerTime = field.varint
		case 6:
			if field.typeID != wireBytes || !validText(field.bytes, maxAccountNameBytes, false) {
				return addWireResult{}, invalidResponse("account name")
			}
			pending.AccountName = string(field.bytes)
		case 7:
			// token_gid is an opaque Steam identifier that nothing here reads as
			// a number. SteamKit types it as a plain string, so general text is
			// accepted rather than decimal digits.
			if field.typeID != wireBytes || !validText(field.bytes, maxTokenGIDBytes, true) {
				return addWireResult{}, invalidResponse("token GID")
			}
			pending.TokenGID = string(field.bytes)
		case 8:
			if field.typeID != wireBytes || len(field.bytes) > 64 {
				return addWireResult{}, invalidResponse("identity secret")
			}
			pending.IdentitySecret = append([]byte(nil), field.bytes...)
		case 9:
			if field.typeID != wireBytes || len(field.bytes) > 64 {
				return addWireResult{}, invalidResponse("secret_1")
			}
			pending.Secret1 = append([]byte(nil), field.bytes...)
		case 10:
			if field.typeID != wireVarint || field.varint > math.MaxInt32 {
				return addWireResult{}, invalidResponse("status")
			}
			result.status = int32(field.varint)
		case 11:
			if field.typeID != wireBytes || !validText(field.bytes, maxPhoneHintBytes, true) {
				return addWireResult{}, invalidResponse("phone number hint")
			}
			pending.PhoneHint = string(field.bytes)
		case 12:
			if field.typeID != wireVarint || field.varint > math.MaxUint8 {
				return addWireResult{}, invalidResponse("confirm type")
			}
			confirm = field.varint
		default:
			continue
		}
	}
	if !decoder.validEnd() {
		return addWireResult{}, invalidResponse("trailing bytes")
	}
	if seen&(uint64(1)<<10) == 0 {
		return addWireResult{}, invalidResponse("no status field")
	}
	if result.status != 1 {
		valid = true
		pending.Destroy()
		result.pending = nil
		return result, nil
	}
	switch confirm {
	case 1:
		pending.Confirmation = ConfirmationSMS
	case 2:
		pending.Confirmation = ConfirmationEmail
	default:
		// Steam has already created the authenticator by this point and has
		// sent the user their confirmation code. Throwing the response away
		// over a confirmation type this build has not heard of would discard
		// the account's secrets while leaving the enrolment half-done on
		// Steam's side. Unknown means "ask the user for the code".
		pending.Confirmation = ConfirmationUnknown
	}
	switch {
	case len(pending.SharedSecret) != 20:
		return addWireResult{}, invalidResponse("shared secret length")
	case len(pending.IdentitySecret) != 20:
		return addWireResult{}, invalidResponse("identity secret length")
	case len(pending.Secret1) != 20:
		return addWireResult{}, invalidResponse("secret_1 length")
	case pending.SerialNumber == 0:
		return addWireResult{}, invalidResponse("no serial number")
	case !validTimestampRange(pending.ServerTime):
		return addWireResult{}, invalidResponse("server time out of range")
	case !validRevocationCode(pending.RevocationCode):
		return addWireResult{}, invalidResponse("revocation code")
	case !bytes.HasPrefix(pending.URI, []byte("otpauth://totp/Steam:")):
		return addWireResult{}, invalidResponse("unexpected otpauth URI")
	}
	valid = true
	return result, nil
}

type finalizeWireResult struct {
	success    bool
	wantMore   bool
	serverTime uint64
	status     int32
}

func unmarshalFinalizeResponse(data []byte) (finalizeWireResult, error) {
	var result finalizeWireResult
	var seen uint64
	decoder := wireDecoder{data: data}
	for {
		field, ok := decoder.next()
		if !ok {
			break
		}
		if field.number > maxKnownFinalizeResponseField {
			continue
		}
		if !markSeen(&seen, field.number) {
			return finalizeWireResult{}, invalidResponse("duplicate field")
		}
		if field.typeID != wireVarint {
			return finalizeWireResult{}, invalidResponse("unexpected wire type")
		}
		switch field.number {
		case 1:
			if field.varint > 1 {
				return finalizeWireResult{}, ErrInvalidResponse
			}
			result.success = field.varint == 1
		case 2:
			if field.varint > 1 {
				return finalizeWireResult{}, ErrInvalidResponse
			}
			result.wantMore = field.varint == 1
		case 3:
			result.serverTime = field.varint
		case 4:
			if field.varint > math.MaxInt32 {
				return finalizeWireResult{}, ErrInvalidResponse
			}
			result.status = int32(field.varint)
		default:
			continue
		}
	}
	switch {
	case !decoder.validEnd():
		return finalizeWireResult{}, invalidResponse("trailing bytes")
	case seen&(uint64(1)<<4) == 0:
		return finalizeWireResult{}, invalidResponse("no status field")
	case result.serverTime != 0 && !validTimestampRange(result.serverTime):
		return finalizeWireResult{}, invalidResponse("server time out of range")
	}
	return result, nil
}

func markSeen(seen *uint64, field uint32) bool {
	mask := uint64(1) << field
	if *seen&mask != 0 {
		return false
	}
	*seen |= mask
	return true
}

func validRevocationCode(value []byte) bool {
	if len(value) < 6 || len(value) > 32 || value[0] != 'R' {
		return false
	}
	for _, b := range value[1:] {
		if !((b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z')) {
			return false
		}
	}
	return true
}

func validTimestampRange(value uint64) bool { return value >= minUnixTime && value <= maxUnixTime }
