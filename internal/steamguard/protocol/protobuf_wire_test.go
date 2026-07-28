package protocol

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestAuthenticationRequestWireVectors(t *testing.T) {
	t.Parallel()

	session := testSession()
	tests := []struct {
		name string
		got  []byte
		want string
	}{
		{
			name: "guard code",
			got:  marshalSteamGuardCodeRequest(session, "ABCDE", ChallengeDeviceCode),
			want: "089b2011004c5e02010010011a0541424344452003",
		},
		{
			name: "poll",
			got:  marshalPollRequest(session),
			want: "089b20120401020304",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			want, err := hex.DecodeString(test.want)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(test.got, want) {
				t.Fatalf("wire = %x, want %x", test.got, want)
			}
		})
	}
}

func TestProtobufDecoderRejectsMalformedWire(t *testing.T) {
	t.Parallel()

	tests := [][]byte{
		{0x08, 0x80, 0x00},
		{0x08, 0x80},
		{0x0b},
		{0x12, 0x02, 0x01},
		{0x00},
	}
	for _, input := range tests {
		decoder := protobufDecoder{data: input}
		_, _ = decoder.next()
		if decoder.validEnd() {
			t.Fatalf("decoder accepted malformed wire %x", input)
		}
	}
}
