package basic

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"TcNo-Acc-Switcher/internal/platform"
)

func TestIsREG(t *testing.T) {
	t.Parallel()
	if !isREG("REG:HKLM\\Path") {
		t.Error("REG: should match")
	}
	if !isREG("reg:HKCU\\Path") {
		t.Error("lowercase reg: should match (case-insensitive)")
	}
	if isREG("FILE:path") {
		t.Error("FILE: should not match")
	}
}

func TestIsJSONSelect(t *testing.T) {
	t.Parallel()
	tests := []struct {
		fn    func(string) bool
		input string
		want  bool
	}{
		{isJSONSelect, "JSON_SELECT::path", true},
		{isJSONSelect, "JSON_SELECT_FIRST::path", true},
		{isJSONSelect, "JSON_EMPTY_VALUE::path", false},
		{isJSONSelect, "FILE:path", false},

		{isJSONSelectFirst, "JSON_SELECT_FIRST::path", true},
		{isJSONSelectFirst, "JSON_SELECT::path", false},

		{isJSONSelectLast, "JSON_SELECT_LAST::path", true},
		{isJSONSelectLast, "JSON_SELECT_FIRST::path", false},

		{isJSONEmptyValue, "JSON_EMPTY_VALUE::path", true},
		{isJSONEmptyValue, "JSON_SELECT::path", false},
	}
	for _, tt := range tests {
		if got := tt.fn(tt.input); got != tt.want {
			t.Errorf("%s(%q) = %v, want %v", funcName(tt.fn), tt.input, got, tt.want)
		}
	}
}

func funcName(fn func(string) bool) string {
	return strings.SplitN(fmt.Sprintf("%p", fn), ".", 2)[0]
}

func TestParseJSONSelectWithDelimiter(t *testing.T) {
	t.Parallel()

	path, jp, delim, ok := parseJSONSelectWithDelimiter("JSON_SELECT_FIRST", "JSON_SELECT_FIRST:|::/path/to/file.json::data.users")
	if !ok {
		t.Fatal("parse failed")
	}
	if path != "/path/to/file.json" {
		t.Errorf("path = %q", path)
	}
	if jp != "data.users" {
		t.Errorf("jsonPath = %q", jp)
	}
	if delim != ":|" {
		t.Errorf("delimiter = %q, want :|", delim)
	}

	path2, jp2, delim2, ok2 := parseJSONSelectWithDelimiter("JSON_SELECT_FIRST", "JSON_SELECT_FIRST::/a/b.json::key")
	if !ok2 {
		t.Fatal("parse without delimiter failed")
	}
	if delim2 != "" {
		t.Errorf("delimiter = %q, want empty", delim2)
	}
	_ = path2
	_ = jp2
}

func TestParseJSONSelect(t *testing.T) {
	t.Parallel()
	path, jp, ok := parseJSONSelect("JSON_SELECT_FIRST", "JSON_SELECT_FIRST::/a/b.json::key")
	if !ok || path != "/a/b.json" || jp != "key" {
		t.Errorf("got %q:%q:%v", path, jp, ok)
	}
}

func TestParseJSONSelectPlain(t *testing.T) {
	t.Parallel()
	path, jp, ok := parseJSONSelectPlain("JSON_SELECT", "JSON_SELECT::/a/b.json::key")
	if !ok || path != "/a/b.json" || jp != "key" {
		t.Errorf("got %q:%q:%v", path, jp, ok)
	}
}

func TestUniqueFromFileRegex_NoRegex(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "uid.txt")
	os.WriteFile(p, []byte("  abc123  \n"), 0o644)

	id, err := uniqueFromFileRegex(platform.Descriptor{
		UniqueIdFile:  p,
		UniqueIdRegex: "",
	}, platform.PathTokenContext{})
	if err != nil {
		t.Fatalf("uniqueFromFileRegex: %v", err)
	}
	if id != "abc123" {
		t.Errorf("got %q, want abc123", id)
	}
}

func TestUniqueFromFileRegex_EmailRegex(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "profile.json")
	os.WriteFile(p, []byte(`{"user":"test@example.com","extra":true}`), 0o644)

	id, err := uniqueFromFileRegex(platform.Descriptor{
		UniqueIdFile:  p,
		UniqueIdRegex: "EMAIL_REGEX",
	}, platform.PathTokenContext{})
	if err != nil {
		t.Fatalf("uniqueFromFileRegex: %v", err)
	}
	if id != "test@example.com" {
		t.Errorf("got %q, want test@example.com", id)
	}
}

func TestUniqueFromFileRegex_NoMatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "data.txt")
	os.WriteFile(p, []byte(`no regex match here`), 0o644)

	_, err := uniqueFromFileRegex(platform.Descriptor{
		UniqueIdFile:  p,
		UniqueIdRegex: `[a-z]+@[a-z]+\.[a-z]+`,
	}, platform.PathTokenContext{})
	if err == nil {
		t.Fatal("expected error for no regex match")
	}
}

func TestRegistryCellToUniqueString(t *testing.T) {
	t.Parallel()
	got, _ := registryCellToUniqueString(" hello ", 0)
	if got != "hello" {
		t.Errorf("string: got %q, want hello", got)
	}
	got2, _ := registryCellToUniqueString([]byte{0xAB, 0xCD}, 0)
	if len(got2) != 40 {
		t.Errorf("[]byte: expected 40-char hex SHA1, got %q (len=%d)", got2, len(got2))
	}
}

func TestFlow_JSONSelect_SaveRestore(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()

	jsonPath := filepath.Join(env.instDir, "settings.json")
	os.WriteFile(jsonPath, []byte(`{"activeUser":"player99","settings":{"volume":0.8}}`), 0o644)

	liveKey := "JSON_SELECT::" + jsonPath + "::activeUser"
	fc.Descriptor.LoginFiles = map[string]string{
		liveKey: "Saved/active_user.json",
	}

	if err := saveCurrentAfterKill(FlowDeps{}, "JSONAcct", fc); err != nil {
		t.Fatalf("save: %v", err)
	}

	cached := filepath.Join(env.cacheDir("JSONAcct"), "Saved", "active_user.json")
	if !pathExists(t, cached) {
		t.Fatal("cached JSON_SELECT result not created")
	}
	val := strings.Trim(mustRead(t, cached), `"`+"\n\r\t ")
	if val != "player99" {
		t.Errorf("cached = %q, want player99", val)
	}
}

func TestFlow_JSONSelectFirst(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()

	jsonPath := filepath.Join(env.instDir, "users.json")
	os.WriteFile(jsonPath, []byte(`{"users":[{"id":"u1","name":"Alice"},{"id":"u2","name":"Bob"}]}`), 0o644)

	key := "JSON_SELECT_FIRST::" + jsonPath + "::users.#.id"
	fc.Descriptor.LoginFiles = map[string]string{
		key: "Saved/first_id.json",
	}

	if err := saveCurrentAfterKill(FlowDeps{}, "JSONFirst", fc); err != nil {
		t.Fatalf("save: %v", err)
	}

	cached := filepath.Join(env.cacheDir("JSONFirst"), "Saved", "first_id.json")
	if !pathExists(t, cached) {
		t.Fatal("cached file not created")
	}
	val := strings.Trim(mustRead(t, cached), `"`)
	if val != "u1" {
		t.Errorf("cached = %q, want u1", val)
	}
}

func TestFlow_JSONSelectLast(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()

	jsonPath := filepath.Join(env.instDir, "sessions.json")
	os.WriteFile(jsonPath, []byte(`{"sessions":["2024-01-01","2024-12-31","2025-06-15"]}`), 0o644)

	key := "JSON_SELECT_LAST::" + jsonPath + "::sessions"
	fc.Descriptor.LoginFiles = map[string]string{
		key: "Saved/last_session.json",
	}

	if err := saveCurrentAfterKill(FlowDeps{}, "JSONLast", fc); err != nil {
		t.Fatalf("save: %v", err)
	}

	cached := filepath.Join(env.cacheDir("JSONLast"), "Saved", "last_session.json")
	if !pathExists(t, cached) {
		t.Fatal("cached file not created")
	}
	val := strings.Trim(mustRead(t, cached), `"`)
	if val != "2025-06-15" {
		t.Errorf("cached = %q, want 2025-06-15", val)
	}
}

func TestUniqueFromJSONSelect_Plain(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "profile.json")
	os.WriteFile(jsonPath, []byte(`{"uid":"abc-def-123","name":"Test"}`), 0o644)

	id, err := uniqueFromJSONSelect(platform.Descriptor{
		UniqueIdFile:   fmt.Sprintf("JSON_SELECT::%s::uid", jsonPath),
		UniqueIdMethod: "",
	}, platform.PathTokenContext{})
	if err != nil {
		t.Fatalf("uniqueFromJSONSelect: %v", err)
	}
	if id != "abc-def-123" {
		t.Errorf("got %q, want abc-def-123", id)
	}
}

func TestUniqueFromJSONSelect_First(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "data.json")
	os.WriteFile(jsonPath, []byte(`{"ids":[101,202,303]}`), 0o644)

	id, err := uniqueFromJSONSelect(platform.Descriptor{
		UniqueIdFile:   fmt.Sprintf("JSON_SELECT_FIRST::%s::ids", jsonPath),
		UniqueIdMethod: "",
	}, platform.PathTokenContext{})
	if err != nil {
		t.Fatalf("uniqueFromJSONSelect: %v", err)
	}
	if id != "101" {
		t.Errorf("got %q, want 101", id)
	}
}

func TestUniqueFromJSONSelect_Last(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "data.json")
	os.WriteFile(jsonPath, []byte(`{"ids":[101,202,303]}`), 0o644)

	id, err := uniqueFromJSONSelect(platform.Descriptor{
		UniqueIdFile:   fmt.Sprintf("JSON_SELECT_LAST::%s::ids", jsonPath),
		UniqueIdMethod: "",
	}, platform.PathTokenContext{})
	if err != nil {
		t.Fatalf("uniqueFromJSONSelect: %v", err)
	}
	if id != "303" {
		t.Errorf("got %q, want 303", id)
	}
}
