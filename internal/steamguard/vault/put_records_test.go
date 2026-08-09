package vault

import (
	"bytes"
	"errors"
	"testing"
)

func recordFor(t *testing.T, v *Vault, steamID64 string) []byte {
	t.Helper()
	records, err := v.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, record := range records {
		if record.SteamID64 != steamID64 {
			continue
		}
		raw, err := v.GetRecord(record.ID)
		if err != nil {
			t.Fatalf("GetRecord(%s): %v", steamID64, err)
		}
		return raw
	}
	t.Fatalf("no record for %s", steamID64)
	return nil
}

// The point of the batch: refreshing a whole vault's access tokens must cost
// one generation rotation, not one per account. A rotation invalidates every
// outstanding capability, so the count is the behaviour under test.
func TestPutRecordsCommitsOneGenerationForTheWholeBatch(t *testing.T) {
	v := newUnlocked(t)
	before := v.Generation()

	updates := []RecordUpdate{
		{SteamID64: "76561198000000001", Plaintext: []byte(`{"a":1}`)},
		{SteamID64: "76561198000000002", Plaintext: []byte(`{"b":2}`)},
		{SteamID64: "76561198000000003", Plaintext: []byte(`{"c":3}`)},
	}
	if err := v.PutRecords(updates); err != nil {
		t.Fatalf("PutRecords: %v", err)
	}

	after := v.Generation()
	if after == before {
		t.Fatal("generation did not change")
	}
	records, err := v.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != len(updates) {
		t.Fatalf("listed %d records, want %d", len(records), len(updates))
	}
	for _, update := range updates {
		if got := recordFor(t, v, update.SteamID64); !bytes.Equal(got, update.Plaintext) {
			t.Fatalf("record %s = %q, want %q", update.SteamID64, got, update.Plaintext)
		}
	}

	// A second batch is one more rotation, not three.
	if err := v.PutRecords(updates); err != nil {
		t.Fatalf("second PutRecords: %v", err)
	}
	if v.Generation() == after {
		t.Fatal("second batch did not rotate the generation")
	}
}

func TestPutRecordsReplacesAndInsertsTogether(t *testing.T) {
	v := newUnlocked(t)
	if _, err := v.PutRecord("76561198000000001", []byte(`{"old":true}`)); err != nil {
		t.Fatalf("PutRecord: %v", err)
	}

	if err := v.PutRecords([]RecordUpdate{
		{SteamID64: "76561198000000001", Plaintext: []byte(`{"new":true}`)},
		{SteamID64: "76561198000000002", Plaintext: []byte(`{"fresh":true}`)},
	}); err != nil {
		t.Fatalf("PutRecords: %v", err)
	}

	records, err := v.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("listed %d records, want 2", len(records))
	}
	if got := recordFor(t, v, "76561198000000001"); !bytes.Equal(got, []byte(`{"new":true}`)) {
		t.Fatalf("replaced record = %q", got)
	}
}

// A duplicate would stage two record files for one account and leave the
// keyring pointing at one of them, orphaning the other's ciphertext.
func TestPutRecordsRejectsADuplicateAccount(t *testing.T) {
	v := newUnlocked(t)
	before := v.Generation()
	err := v.PutRecords([]RecordUpdate{
		{SteamID64: "76561198000000001", Plaintext: []byte(`{"a":1}`)},
		{SteamID64: "76561198000000001", Plaintext: []byte(`{"a":2}`)},
	})
	if !errors.Is(err, ErrInvalidFormat) {
		t.Fatalf("err = %v, want ErrInvalidFormat", err)
	}
	if v.Generation() != before {
		t.Fatal("a rejected batch rotated the generation")
	}
}

// One unusable update must not half-apply the rest.
func TestPutRecordsRejectsTheWholeBatchBeforeWritingAnything(t *testing.T) {
	v := newUnlocked(t)
	if _, err := v.PutRecord("76561198000000001", []byte(`{"old":true}`)); err != nil {
		t.Fatalf("PutRecord: %v", err)
	}
	before := v.Generation()

	err := v.PutRecords([]RecordUpdate{
		{SteamID64: "76561198000000001", Plaintext: []byte(`{"new":true}`)},
		{SteamID64: "", Plaintext: []byte(`{"nameless":true}`)},
	})
	if !errors.Is(err, ErrInvalidFormat) {
		t.Fatalf("err = %v, want ErrInvalidFormat", err)
	}
	if v.Generation() != before {
		t.Fatal("a rejected batch rotated the generation")
	}
	if got := recordFor(t, v, "76561198000000001"); !bytes.Equal(got, []byte(`{"old":true}`)) {
		t.Fatalf("record = %q, want the pre-batch value", got)
	}
}

func TestPutRecordsAcceptsAnEmptyBatch(t *testing.T) {
	v := newUnlocked(t)
	before := v.Generation()
	if err := v.PutRecords(nil); err != nil {
		t.Fatalf("PutRecords(nil): %v", err)
	}
	if v.Generation() != before {
		t.Fatal("an empty batch rotated the generation")
	}
}
