package protocol

import (
	"encoding/binary"
	"math"
)

const (
	wireVarint  = 0
	wireFixed64 = 1
	wireBytes   = 2
	wireFixed32 = 5
)

type protobufField struct {
	number  uint32
	typeID  uint8
	varint  uint64
	fixed64 uint64
	fixed32 uint32
	bytes   []byte
}

type protobufDecoder struct {
	data   []byte
	offset int
}

func (d *protobufDecoder) next() (protobufField, bool) {
	if d.offset == len(d.data) {
		return protobufField{}, false
	}
	tag, size, ok := consumeCanonicalVarint(d.data[d.offset:])
	if !ok {
		d.offset = -1
		return protobufField{}, false
	}
	d.offset += size
	number := uint32(tag >> 3)
	typeID := uint8(tag & 7)
	if number == 0 || number > (1<<29)-1 {
		d.offset = -1
		return protobufField{}, false
	}
	field := protobufField{number: number, typeID: typeID}

	switch typeID {
	case wireVarint:
		value, consumed, valid := consumeCanonicalVarint(d.data[d.offset:])
		if !valid {
			d.offset = -1
			return protobufField{}, false
		}
		d.offset += consumed
		field.varint = value
	case wireFixed64:
		if len(d.data)-d.offset < 8 {
			d.offset = -1
			return protobufField{}, false
		}
		field.fixed64 = binary.LittleEndian.Uint64(d.data[d.offset : d.offset+8])
		d.offset += 8
	case wireBytes:
		length, consumed, valid := consumeCanonicalVarint(d.data[d.offset:])
		if !valid {
			d.offset = -1
			return protobufField{}, false
		}
		d.offset += consumed
		remaining := len(d.data) - d.offset
		if length > uint64(remaining) {
			d.offset = -1
			return protobufField{}, false
		}
		field.bytes = d.data[d.offset : d.offset+int(length)]
		d.offset += int(length)
	case wireFixed32:
		if len(d.data)-d.offset < 4 {
			d.offset = -1
			return protobufField{}, false
		}
		field.fixed32 = binary.LittleEndian.Uint32(d.data[d.offset : d.offset+4])
		d.offset += 4
	default:
		d.offset = -1
		return protobufField{}, false
	}
	return field, true
}

func (d *protobufDecoder) validEnd() bool {
	return d.offset == len(d.data)
}

func consumeCanonicalVarint(data []byte) (uint64, int, bool) {
	value, size := binary.Uvarint(data)
	if size <= 0 || size != protobufVarintSize(value) {
		return 0, 0, false
	}
	return value, size, true
}

func protobufVarintSize(value uint64) int {
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

func appendVarintField(dst []byte, number uint32, value uint64) []byte {
	dst = appendTag(dst, number, wireVarint)
	return binary.AppendUvarint(dst, value)
}

func appendFixed64Field(dst []byte, number uint32, value uint64) []byte {
	dst = appendTag(dst, number, wireFixed64)
	return binary.LittleEndian.AppendUint64(dst, value)
}

func appendFixed32Field(dst []byte, number uint32, value uint32) []byte {
	dst = appendTag(dst, number, wireFixed32)
	return binary.LittleEndian.AppendUint32(dst, value)
}

func appendFloat32Field(dst []byte, number uint32, value float32) []byte {
	return appendFixed32Field(dst, number, math.Float32bits(value))
}

func appendBytesField(dst []byte, number uint32, value []byte) []byte {
	dst = appendTag(dst, number, wireBytes)
	dst = binary.AppendUvarint(dst, uint64(len(value)))
	return append(dst, value...)
}

func appendStringField(dst []byte, number uint32, value string) []byte {
	return appendBytesField(dst, number, []byte(value))
}

func markSingleton(seen *uint64, field uint32) bool {
	if field == 0 || field >= 64 {
		return false
	}
	mask := uint64(1) << field
	if *seen&mask != 0 {
		return false
	}
	*seen |= mask
	return true
}

func fieldVarint(field protobufField) (uint64, bool) {
	return field.varint, field.typeID == wireVarint
}

func fieldFixed64(field protobufField) (uint64, bool) {
	return field.fixed64, field.typeID == wireFixed64
}

func fieldBytes(field protobufField, max int) ([]byte, bool) {
	return field.bytes, field.typeID == wireBytes && len(field.bytes) <= max
}

func fieldString(field protobufField, max int) (string, bool) {
	value, ok := fieldBytes(field, max)
	if !ok || !validProtocolText(value, max, true) {
		return "", false
	}
	return string(value), true
}
