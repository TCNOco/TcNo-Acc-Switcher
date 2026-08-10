package steam

import (
	"crypto/md5"
	"encoding/binary"
	"math/bits"
)

// cs2FriendCodeAlphabet is the base-32 alphabet the game encodes friend codes
// with. I, O, 0 and 1 are absent so no two characters can be misread.
const cs2FriendCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// cs2FriendCodeHash reproduces the game's mix: MD5 over the little-endian
// account ID tagged with "CSGO", keeping the first four bytes.
func cs2FriendCodeHash(accountID uint32) uint32 {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(accountID)|0x4353474F00000000)
	sum := md5.Sum(buf[:])
	return binary.LittleEndian.Uint32(sum[:4])
}

// CS2FriendCode renders the in-game friend code (e.g. "SUCVS-FADA") for a Steam
// account ID - the same 32-bit value as SteamID32.
//
// The account ID's eight nibbles are interleaved with eight bits of the hash,
// which is what makes the code un-guessable from the ID alone. That yields 13
// five-bit groups; for an individual account the leading group is always "AAAA",
// which the game strips, leaving the familiar 5-4 form.
func CS2FriendCode(accountID uint32) string {
	hash := cs2FriendCodeHash(accountID)

	var r uint64
	id := uint64(accountID)
	for i := 0; i < 8; i++ {
		nibble := id & 0xF
		id >>= 4
		// The high bits deliberately overlap on each pass: this mirrors the
		// game's 64-bit arithmetic, and the overlap is what the decoder undoes.
		a := r<<4 | nibble
		r = r>>28<<32 | a
		r = r>>31<<32 | a<<1 | uint64((hash>>uint(i))&1)
	}

	v := bits.ReverseBytes64(r)
	code := make([]byte, 0, 15)
	for i := 0; i < 13; i++ {
		if i == 4 || i == 9 {
			code = append(code, '-')
		}
		code = append(code, cs2FriendCodeAlphabet[v&0x1F])
		v >>= 5
	}

	if string(code[:4]) == "AAAA" {
		return string(code[5:])
	}
	return string(code)
}
