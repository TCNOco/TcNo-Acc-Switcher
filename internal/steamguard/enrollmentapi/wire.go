package enrollmentapi

import (
	"bytes"
	"encoding/binary"
	"math"
)

const (
	wireVarint  = 0
	wireFixed64 = 1
	wireBytes   = 2
)

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

func marshalFinalizeRequest(steamID uint64, authenticatorCode []byte, authenticatorTime uint64, confirmationCode []byte) []byte {
	message := make([]byte, 0, 80)
	message = appendFixed64(message, 1, steamID)
	message = appendBytes(message, 2, authenticatorCode)
	message = appendVarint(message, 3, authenticatorTime)
	message = appendBytes(message, 4, confirmationCode)
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
		if !markSeen(&seen, field.number) {
			return addWireResult{}, ErrInvalidResponse
		}
		switch field.number {
		case 1:
			if field.typeID != wireBytes || len(field.bytes) > 64 {
				return addWireResult{}, ErrInvalidResponse
			}
			pending.SharedSecret = append([]byte(nil), field.bytes...)
		case 2:
			if field.typeID != wireFixed64 {
				return addWireResult{}, ErrInvalidResponse
			}
			pending.SerialNumber = field.fixed64
		case 3:
			if field.typeID != wireBytes || !validText(field.bytes, 32, false) {
				return addWireResult{}, ErrInvalidResponse
			}
			pending.RevocationCode = append([]byte(nil), field.bytes...)
		case 4:
			if field.typeID != wireBytes || !validText(field.bytes, maxURIBytes, false) {
				return addWireResult{}, ErrInvalidResponse
			}
			pending.URI = append([]byte(nil), field.bytes...)
		case 5:
			if field.typeID != wireVarint {
				return addWireResult{}, ErrInvalidResponse
			}
			pending.ServerTime = field.varint
		case 6:
			if field.typeID != wireBytes || !validText(field.bytes, maxAccountNameBytes, false) {
				return addWireResult{}, ErrInvalidResponse
			}
			pending.AccountName = string(field.bytes)
		case 7:
			if field.typeID != wireBytes || !digits(field.bytes, maxTokenGIDBytes) {
				return addWireResult{}, ErrInvalidResponse
			}
			pending.TokenGID = string(field.bytes)
		case 8:
			if field.typeID != wireBytes || len(field.bytes) > 64 {
				return addWireResult{}, ErrInvalidResponse
			}
			pending.IdentitySecret = append([]byte(nil), field.bytes...)
		case 9:
			if field.typeID != wireBytes || len(field.bytes) > 64 {
				return addWireResult{}, ErrInvalidResponse
			}
			pending.Secret1 = append([]byte(nil), field.bytes...)
		case 10:
			if field.typeID != wireVarint || field.varint > math.MaxInt32 {
				return addWireResult{}, ErrInvalidResponse
			}
			result.status = int32(field.varint)
		case 11:
			if field.typeID != wireBytes || !validText(field.bytes, maxPhoneHintBytes, true) {
				return addWireResult{}, ErrInvalidResponse
			}
			pending.PhoneHint = string(field.bytes)
		case 12:
			if field.typeID != wireVarint || field.varint > math.MaxUint8 {
				return addWireResult{}, ErrInvalidResponse
			}
			confirm = field.varint
		default:
			return addWireResult{}, ErrInvalidResponse
		}
	}
	if !decoder.validEnd() || seen&(uint64(1)<<10) == 0 {
		return addWireResult{}, ErrInvalidResponse
	}
	if result.status != 1 {
		valid = true
		pending.Destroy()
		result.pending = nil
		return result, nil
	}
	switch confirm {
	case 0:
		pending.Confirmation = ConfirmationUnknown
	case 1:
		pending.Confirmation = ConfirmationSMS
	case 2:
		pending.Confirmation = ConfirmationEmail
	default:
		return addWireResult{}, ErrInvalidResponse
	}
	if len(pending.SharedSecret) != 20 || len(pending.IdentitySecret) != 20 || len(pending.Secret1) != 20 ||
		pending.SerialNumber == 0 || !validTimestampRange(pending.ServerTime) ||
		!validRevocationCode(pending.RevocationCode) || !bytes.HasPrefix(pending.URI, []byte("otpauth://totp/Steam:")) {
		return addWireResult{}, ErrInvalidResponse
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
		if !markSeen(&seen, field.number) || field.typeID != wireVarint {
			return finalizeWireResult{}, ErrInvalidResponse
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
			return finalizeWireResult{}, ErrInvalidResponse
		}
	}
	if !decoder.validEnd() || seen&(uint64(1)<<4) == 0 || (result.serverTime != 0 && !validTimestampRange(result.serverTime)) {
		return finalizeWireResult{}, ErrInvalidResponse
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

func digits(value []byte, max int) bool {
	if len(value) == 0 || len(value) > max {
		return false
	}
	for _, b := range value {
		if b < '0' || b > '9' {
			return false
		}
	}
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
