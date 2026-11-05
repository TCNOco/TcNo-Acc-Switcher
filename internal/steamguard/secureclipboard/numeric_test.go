package secureclipboard

import "testing"

func TestNumericCodesRemainSeparateFromSteamCodes(t *testing.T) {
	for _, value := range []string{"07081804", "00000000", "99999999"} {
		if _, err := parseNumericCode(value); err != nil {
			t.Fatal(err)
		}
		if _, err := parseCode(value); err == nil {
			t.Fatal("numeric code accepted as Steam code")
		}
	}
	for _, value := range []string{"1234567", "123456789", "12A45678", "１２３４５６７８"} {
		if _, err := parseNumericCode(value); err == nil {
			t.Fatal("invalid numeric code accepted")
		}
	}
	if _, err := parseCode("23456"); err != nil {
		t.Fatal("existing Steam code rejected")
	}
}
