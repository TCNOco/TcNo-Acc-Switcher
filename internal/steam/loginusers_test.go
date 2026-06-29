package steam

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEscapeVDF(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, want string }{
		{"hello", "hello"},
		{`back\slash`, `back\\slash`},
		{`quote"me`, `quote\"me`},
		{`both\and"`, `both\\and\"`},
		{"", ""},
	}
	for _, tt := range tests {
		got := escapeVDF(tt.in)
		if got != tt.want {
			t.Errorf("escapeVDF(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestLooksLikeSteamID64(t *testing.T) {
	t.Parallel()
	if !looksLikeSteamID64("76561198000000000") {
		t.Error("valid SteamID64 should match")
	}
	if looksLikeSteamID64("") {
		t.Error("empty should not match")
	}
	if looksLikeSteamID64("abc") {
		t.Error("non-numeric should not match")
	}
	if looksLikeSteamID64("12345") {
		t.Error("5-digit should not match")
	}
}

func TestActiveSessionSteamID64(t *testing.T) {
	t.Parallel()

	t.Run("single_most_recent", func(t *testing.T) {
		users := []LoginUser{
			{SteamID64: "111", MostRecent: "0"},
			{SteamID64: "222", MostRecent: "1"},
			{SteamID64: "333", MostRecent: "0"},
		}
		if got := ActiveSessionSteamID64(users); got != "222" {
			t.Errorf("got %q, want 222", got)
		}
	})

	t.Run("zero_most_recent", func(t *testing.T) {
		users := []LoginUser{
			{SteamID64: "111", MostRecent: "0"},
			{SteamID64: "222", MostRecent: "0"},
		}
		if got := ActiveSessionSteamID64(users); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("multiple_most_recent", func(t *testing.T) {
		users := []LoginUser{
			{SteamID64: "111", MostRecent: "1"},
			{SteamID64: "222", MostRecent: "1"},
		}
		if got := ActiveSessionSteamID64(users); got != "" {
			t.Errorf("got %q, want empty (ambiguous)", got)
		}
	})

	t.Run("empty_slice", func(t *testing.T) {
		if got := ActiveSessionSteamID64(nil); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("whitespace_most_recent", func(t *testing.T) {
		users := []LoginUser{
			{SteamID64: "111", MostRecent: " 1 "},
		}
		if got := ActiveSessionSteamID64(users); got != "111" {
			t.Errorf("got %q, want 111", got)
		}
	})
}

func TestParseLoginUsers_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loginusers.vdf")

	vdfContent := `"users"
{
	"76561198000000001"
	{
		"AccountName"		"player1"
		"PersonaName"		"Player One"
		"Timestamp"		"1700000000"
		"WantsOfflineMode"		"0"
		"MostRecent"		"1"
		"RememberPassword"		"1"
	}
	"76561198000000002"
	{
		"AccountName"		"player2"
		"PersonaName"		"Player Two"
		"Timestamp"		"1690000000"
		"WantsOfflineMode"		"0"
		"MostRecent"		"0"
		"RememberPassword"		"0"
	}
}
`
	os.WriteFile(path, []byte(vdfContent), 0o644)

	users, err := ParseLoginUsers(path)
	if err != nil {
		t.Fatalf("ParseLoginUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}

	if users[0].SteamID64 != "76561198000000001" {
		t.Errorf("user 0 ID = %q", users[0].SteamID64)
	}
	if users[0].PersonaName != "Player One" {
		t.Errorf("user 0 persona = %q", users[0].PersonaName)
	}
	if users[0].AccountName != "player1" {
		t.Errorf("user 0 account = %q", users[0].AccountName)
	}
	if users[0].MostRecent != "1" {
		t.Errorf("user 0 MostRecent = %q, want 1", users[0].MostRecent)
	}
	if users[0].RememberPassword != "1" {
		t.Errorf("user 0 RememberPassword = %q, want 1", users[0].RememberPassword)
	}

	if users[1].SteamID64 != "76561198000000002" {
		t.Errorf("user 1 ID = %q", users[1].SteamID64)
	}
	if users[1].MostRecent != "0" {
		t.Errorf("user 1 MostRecent = %q", users[1].MostRecent)
	}
}

func TestParseLoginUsers_SkipOffline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loginusers.vdf")

	vdfContent := `"users"
{
	"76561198000000003"
	{
		"AccountName"		"skipper"
		"PersonaName"		"Skip"
		"Timestamp"		"1700000000"
		"SkipOfflineModeWarning"		"1"
	}
}
`
	os.WriteFile(path, []byte(vdfContent), 0o644)

	users, _ := ParseLoginUsers(path)
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].SkipOfflineWarn != "1" {
		t.Errorf("SkipOfflineWarn = %q, want 1", users[0].SkipOfflineWarn)
	}
}

func TestParseLoginUsers_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loginusers.vdf")

	vdfContent := `"users"
{
	"76561198000000004"
	{
		"accountname"		"casey"
		"personaname"		"Casey"
		"mostrecent"		"1"
		"rememberpassword"		"1"
	}
}
`
	os.WriteFile(path, []byte(vdfContent), 0o644)

	users, _ := ParseLoginUsers(path)
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].AccountName != "casey" {
		t.Errorf("AccountName = %q", users[0].AccountName)
	}
	if users[0].PersonaName != "Casey" {
		t.Errorf("PersonaName = %q", users[0].PersonaName)
	}
	if users[0].MostRecent != "1" {
		t.Errorf("MostRecent = %q, want 1", users[0].MostRecent)
	}
	if users[0].RememberPassword != "1" {
		t.Errorf("RememberPassword = %q, want 1", users[0].RememberPassword)
	}
}

func TestParseLoginUsers_VDFLastFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loginusers.vdf")
	altPath := filepath.Join(dir, "loginusers.vdf_last")

	os.WriteFile(path, []byte(`"users"\n{\n}\n`), 0o644)
	os.WriteFile(altPath, []byte(`"users"
{
	"76561198000000200"
	{
		"AccountName"		"fallback"
		"PersonaName"		"Fallback"
		"Timestamp"		"1"
	}
}
`), 0o644)

	users, err := ParseLoginUsers(path)
	if err != nil {
		t.Fatalf("ParseLoginUsers: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user from .vdf_last, got %d", len(users))
	}
	if users[0].AccountName != "fallback" {
		t.Errorf("AccountName = %q", users[0].AccountName)
	}
}

func TestParseLoginUsers_BOM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loginusers.vdf")

	vdfContent := `"users"
{
	"76561198000000030"
	{
		"AccountName"		"bommer"
		"PersonaName"		"BOM"
		"Timestamp"		"1"
	}
}
`
	data := append([]byte{0xef, 0xbb, 0xbf}, []byte(vdfContent)...)
	os.WriteFile(path, data, 0o644)

	users, _ := ParseLoginUsers(path)
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].AccountName != "bommer" {
		t.Errorf("AccountName = %q", users[0].AccountName)
	}
}

func TestParseLoginUsers_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loginusers.vdf")
	os.WriteFile(path, []byte{}, 0o644)

	users, err := ParseLoginUsers(path)
	if err != nil {
		t.Fatalf("ParseLoginUsers: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("expected 0 users, got %d", len(users))
	}
}

func TestParseLoginUsers_MissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loginusers.vdf")

	_, err := ParseLoginUsers(path)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoginUsers_SerializeRoundtrip(t *testing.T) {
	t.Parallel()
	users := []LoginUser{
		{SteamID64: "76561198000000100", AccountName: "acct1", PersonaName: "One", Timestamp: "1000", WantsOffline: "0", MostRecent: "1", RememberPassword: "1"},
		{SteamID64: "76561198000000200", AccountName: "acct2", PersonaName: "Two", Timestamp: "2000", WantsOffline: "1", MostRecent: "0", RememberPassword: "0", SkipOfflineWarn: "1"},
	}

	kv := LoginUsersToKeyValue(users)
	text := KeyValueToText(kv)

	dir := t.TempDir()
	path := filepath.Join(dir, "loginusers.vdf")
	os.WriteFile(path, text, 0o644)

	parsed, err := ParseLoginUsers(path)
	if err != nil {
		t.Fatalf("ParseLoginUsers: %v", err)
	}
	if len(parsed) != 2 {
		t.Fatalf("expected 2 users, got %d", len(parsed))
	}

	for i, u := range users {
		if parsed[i].SteamID64 != u.SteamID64 {
			t.Errorf("user %d: ID = %q, want %q", i, parsed[i].SteamID64, u.SteamID64)
		}
		if parsed[i].AccountName != u.AccountName {
			t.Errorf("user %d: AccountName = %q", i, parsed[i].AccountName)
		}
		if parsed[i].PersonaName != u.PersonaName {
			t.Errorf("user %d: PersonaName = %q", i, parsed[i].PersonaName)
		}
		if parsed[i].MostRecent != u.MostRecent {
			t.Errorf("user %d: MostRecent = %q", i, parsed[i].MostRecent)
		}
		if parsed[i].RememberPassword != u.RememberPassword {
			t.Errorf("user %d: RememberPassword = %q", i, parsed[i].RememberPassword)
		}
		if parsed[i].SkipOfflineWarn != u.SkipOfflineWarn {
			t.Errorf("user %d: SkipOfflineWarn = %q", i, parsed[i].SkipOfflineWarn)
		}
	}
}

func TestLoginUsersToKeyValue_Empty(t *testing.T) {
	t.Parallel()
	kv := LoginUsersToKeyValue(nil)
	if kv.Key != "users" {
		t.Errorf("key = %q, want users", kv.Key)
	}
	if len(kv.Children) != 0 {
		t.Errorf("expected 0 children, got %d", len(kv.Children))
	}

	kv2 := LoginUsersToKeyValue([]LoginUser{{SteamID64: "", AccountName: "ghost"}})
	if len(kv2.Children) != 0 {
		t.Errorf("empty SteamID64 should be filtered, got %d children", len(kv2.Children))
	}
}

func TestLoginUsers_SerializeSpecialChars(t *testing.T) {
	t.Parallel()
	users := []LoginUser{
		{SteamID64: "76561198000000100", AccountName: "normaluser", PersonaName: "Normal Name", Timestamp: "1", MostRecent: "1"},
	}

	kv := LoginUsersToKeyValue(users)
	text := string(KeyValueToText(kv))

	if !strings.Contains(text, "PersonaName") {
		t.Error("missing PersonaName in output")
	}
	if !strings.Contains(text, "76561198000000100") {
		t.Error("missing SteamID64 in output")
	}
	if !strings.Contains(text, `"users"`) {
		t.Error("missing users wrapper")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "loginusers.vdf")
	os.WriteFile(path, []byte(text), 0o644)

	parsed, _ := ParseLoginUsers(path)
	if len(parsed) != 1 {
		t.Fatalf("expected 1 user, got %d", len(parsed))
	}
	if parsed[0].MostRecent != "1" {
		t.Errorf("MostRecent = %q, want 1", parsed[0].MostRecent)
	}
	if parsed[0].AccountName != "normaluser" {
		t.Errorf("AccountName = %q", parsed[0].AccountName)
	}
}

func TestSetShowSteamSwitcher(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	os.MkdirAll(configDir, 0o755)

	configVDF := `"Store"
{
	"Software"
	{
		"Valve"
		{
			"Steam"
			{
				"AlwaysShowUserChooser"		"0"
			}
		}
	}
}
`
	configPath := filepath.Join(configDir, "config.vdf")
	os.WriteFile(configPath, []byte(configVDF), 0o644)

	if err := setShowSteamSwitcher(dir, true); err != nil {
		t.Fatalf("setShowSteamSwitcher(true): %v", err)
	}

	data, _ := os.ReadFile(configPath)
	content := string(data)
	if !strings.Contains(content, `"AlwaysShowUserChooser"		"1"`) {
		t.Errorf("expected AlwaysShowUserChooser = 1, got: %s", content)
	}

	if err := setShowSteamSwitcher(dir, false); err != nil {
		t.Fatalf("setShowSteamSwitcher(false): %v", err)
	}
	data, _ = os.ReadFile(configPath)
	content = string(data)
	if !strings.Contains(content, `"AlwaysShowUserChooser"		"0"`) {
		t.Errorf("expected AlwaysShowUserChooser = 0, got: %s", content)
	}
}

func TestSetShowSteamSwitcher_NoExistingLine(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	os.MkdirAll(configDir, 0o755)

	os.WriteFile(filepath.Join(configDir, "config.vdf"), []byte(`"Store"\n{\n}\n`), 0o644)

	err := setShowSteamSwitcher(dir, true)
	if err != nil {
		t.Fatalf("setShowSteamSwitcher: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(configDir, "config.vdf"))
	if strings.Contains(string(data), "AlwaysShowUserChooser") {
		t.Error("AlwaysShowUserChooser should not have been added when missing")
	}
}
