// Package wire hand-encodes the two byte formats gRPC relies on: protobuf
// varints and tags, and the 5-byte gRPC message prefix. Production code uses
// google.golang.org/protobuf; this package exists so the guide can show and
// test every byte.
package wire

import (
	"encoding/binary"
	"errors"
)

// Protobuf wire types (the low 3 bits of a field tag).
const (
	TypeVarint = 0 // int32, int64, uint32, uint64, sint32, sint64, bool, enum
	TypeI64    = 1 // fixed64, sfixed64, double
	TypeLen    = 2 // string, bytes, embedded messages, packed repeated fields
	TypeI32    = 5 // fixed32, sfixed32, float
)

// AppendVarint appends v using 7 data bits per byte, least significant group
// first. The high bit of each byte says whether another byte follows.
func AppendVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

// Tag builds a field key: field number shifted left 3, OR the wire type.
func Tag(field int32, wireType int) uint64 {
	return uint64(field)<<3 | uint64(wireType)
}

// ZigZag32 maps signed to unsigned so small negatives stay small:
// 0->0, -1->1, 1->2, -2->3. sint32 fields use it; int32 fields do not.
func ZigZag32(n int32) uint32 {
	return uint32(n<<1) ^ uint32(n>>31)
}

// ZigZag64 is the 64-bit form used by sint64.
func ZigZag64(n int64) uint64 {
	return uint64(n<<1) ^ uint64(n>>63)
}

// Frame prefixes a serialized message the way gRPC does inside HTTP/2 DATA
// frames: 1 byte compressed flag, 4 bytes big-endian length, then payload.
func Frame(payload []byte, compressed bool) []byte {
	out := make([]byte, 5, 5+len(payload))
	if compressed {
		out[0] = 1
	}
	binary.BigEndian.PutUint32(out[1:5], uint32(len(payload)))
	return append(out, payload...)
}

// ErrShortFrame is returned when the buffer ends before a full message.
var ErrShortFrame = errors.New("wire: incomplete gRPC frame")

// Unframe reads one length-prefixed message and returns the remaining bytes,
// which may hold further messages: one DATA frame can carry several.
func Unframe(b []byte) (payload []byte, compressed bool, rest []byte, err error) {
	if len(b) < 5 {
		return nil, false, b, ErrShortFrame
	}
	n := binary.BigEndian.Uint32(b[1:5])
	if uint64(len(b)-5) < uint64(n) {
		return nil, false, b, ErrShortFrame
	}
	end := 5 + int(n)
	return b[5:end], b[0] == 1, b[end:], nil
}
