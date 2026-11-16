package accountindex

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestReadFileMigratesDotNetIndexOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ids.json")
	original := []byte("\xef\xbb\xbf" + `{"first@example.test":"Player One","second@example.test":"Player Two"}`)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	// Startup counts, the account page and encryption can discover an old
	// index concurrently. They must all see the complete migrated dictionary.
	var readers sync.WaitGroup
	for range 8 {
		readers.Go(func() {
			data, err := ReadFile(path)
			if err != nil {
				t.Error(err)
				return
			}
			var got struct {
				IDs map[string]string `json:"ids"`
			}
			if err := json.Unmarshal(data, &got); err != nil {
				t.Error(err)
			}
			want := map[string]string{"first@example.test": "Player One", "second@example.test": "Player Two"}
			if !reflect.DeepEqual(got.IDs, want) {
				t.Errorf("migrated IDs = %v, want %v", got.IDs, want)
			}
		})
	}
	readers.Wait()
	backup, err := os.ReadFile(path + ".legacy-dotnet.bak")
	if err != nil || !bytes.Equal(backup, original) {
		t.Fatalf("backup = %q, err = %v", backup, err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(path); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil || !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("second read rewrote the migrated index: %v", err)
	}
}

func TestReadFilePreservesCurrentIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ids.json")
	original := []byte(`{"ids":{"a":"Player"},"lastused":{"a":"2026-01-01T00:00:00Z"},"tags":{"t":{"name":"Keep"}},"accountTags":{"a":["t"]},"futureField":true}`)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile(path)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("ReadFile = %q, %v", got, err)
	}
	if _, err := os.Stat(path + ".legacy-dotnet.bak"); !os.IsNotExist(err) {
		t.Fatalf("modern index produced a legacy backup: %v", err)
	}
}

func TestReadFileDoesNotReplaceInvalidIndex(t *testing.T) {
	for _, original := range []string{`{"a":"Player"`, `null`, `[]`, `{"ids":42}`, `{"a":"Player","b":null}`, `{"a":"Player","unknown":{}}`} {
		t.Run(original, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ids.json")
			if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadFile(path); err == nil {
				t.Fatal("invalid index accepted")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != original {
				t.Fatalf("original changed: %q, %v", got, err)
			}
		})
	}
}

func TestReadFileResumesOnlyWithMatchingBackup(t *testing.T) {
	for _, matching := range []bool{true, false} {
		path := filepath.Join(t.TempDir(), "ids.json")
		original := []byte(`{"a":"Player"}`)
		if err := os.WriteFile(path, original, 0o644); err != nil {
			t.Fatal(err)
		}
		backup := original
		if !matching {
			backup = []byte(`{"other":"Preserve this backup"}`)
		}
		if err := os.WriteFile(path+".legacy-dotnet.bak", backup, 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := ReadFile(path)
		if (err == nil) != matching {
			t.Fatalf("matching=%v: error=%v", matching, err)
		}
		got, err := os.ReadFile(path + ".legacy-dotnet.bak")
		if err != nil || !bytes.Equal(got, backup) {
			t.Fatalf("backup changed: %q, %v", got, err)
		}
		if !matching {
			got, err = os.ReadFile(path)
			if err != nil || !bytes.Equal(got, original) {
				t.Fatalf("original changed: %q, %v", got, err)
			}
		}
	}
}

func TestReadFileBackupFailurePreservesLegacyIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ids.json")
	original := []byte(`{"a":"Player"}`)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+".legacy-dotnet.bak", 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(path); err == nil {
		t.Fatal("migration succeeded without a backup")
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("original changed: %q, %v", got, err)
	}
}
