package basic

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestSelectDescriptorJSON(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
		path string
		want string
	}{
		{name: "account ID", data: `{"account_id":"abc123-456"}`, path: "account_id", want: "abc123-456"},
		{name: "display name", data: `{"prod":{"account_info":{"display_name":"  Player One  "}}}`, path: "prod.account_info.display_name", want: "Player One"},
		{name: "UTF8 BOM", data: "\ufeff" + `{"account_id":"abc"}`, path: "account_id", want: "abc"},
		{name: "escaped string", data: `{"name":"Player \"One\""}`, path: "name", want: `Player "One"`},
		{name: "Unicode", data: `{"name":"玩家"}`, path: "name", want: "玩家"},
		{name: "zero string", data: `{"id":"0"}`, path: "id", want: "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := selectDescriptorJSON([]byte(tc.data), tc.path)
			if err != nil || got != tc.want {
				t.Fatalf("selectDescriptorJSON() = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestSelectDescriptorJSONRejectsInvalidValues(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
		path string
	}{
		{name: "malformed", data: `{"id":"abc"`, path: "id"},
		{name: "trailing JSON", data: `{"id":"abc"} {}`, path: "id"},
		{name: "empty input", path: "id"},
		{name: "missing", data: `{}`, path: "id"},
		{name: "null", data: `{"id":null}`, path: "id"},
		{name: "empty string", data: `{"id":""}`, path: "id"},
		{name: "whitespace", data: `{"id":" \t "}`, path: "id"},
		{name: "number", data: `{"id":123}`, path: "id"},
		{name: "boolean", data: `{"id":true}`, path: "id"},
		{name: "object", data: `{"id":{"value":"abc"}}`, path: "id"},
		{name: "array", data: `{"id":["abc"]}`, path: "id"},
		{name: "empty path", data: `{"id":"abc"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if value, err := selectDescriptorJSON([]byte(tc.data), tc.path); err == nil || value != "" {
				t.Fatalf("selectDescriptorJSON() = %q, %v; expected rejection", value, err)
			}
		})
	}
}

func TestDescriptorJSONQuerySelection(t *testing.T) {
	data := []byte(`{"names":["first","last"],"joined":" first|last ","empty":[],"number":[1]}`)
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{name: "plain", raw: "JSON_SELECT::auth.json::joined", want: "first|last"},
		{name: "first array entry", raw: "JSON_SELECT_FIRST::auth.json::names", want: "first"},
		{name: "last array entry", raw: "JSON_SELECT_LAST::auth.json::names", want: "last"},
		{name: "first delimited entry", raw: "JSON_SELECT_FIRST|::auth.json::joined", want: "first"},
		{name: "last delimited entry", raw: "JSON_SELECT_LAST|::auth.json::joined", want: "last"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query, err := parseDescriptorJSONQuery(tc.raw)
			if err != nil {
				t.Fatal(err)
			}
			if query.filePath != "auth.json" {
				t.Fatalf("file path = %q", query.filePath)
			}
			got, err := query.selectValue(data)
			if err != nil || got != tc.want {
				t.Fatalf("selectValue() = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	for _, raw := range []string{"JSON_SELECT_FIRST::auth.json::empty", "JSON_SELECT_LAST::auth.json::number"} {
		query, err := parseDescriptorJSONQuery(raw)
		if err != nil {
			t.Fatal(err)
		}
		if value, err := query.selectValue(data); err == nil || value != "" {
			t.Fatalf("invalid selection = %q, %v", value, err)
		}
	}
}

func TestDescriptorJSONQueryRejectsInvalidReferences(t *testing.T) {
	for _, raw := range []string{"", "auth.json", "JSON_SELECT::auth.json", "JSON_SELECT::::id", "JSON_SELECT::auth.json::", "JSON_SELECT_OTHER::auth.json::id"} {
		if _, err := parseDescriptorJSONQuery(raw); err == nil {
			t.Errorf("parseDescriptorJSONQuery(%q) succeeded", raw)
		}
	}
}

func TestProtectedJSONUniqueIDValidation(t *testing.T) {
	for _, id := range []string{"abc123-456", strings.Repeat("a", 128)} {
		if err := validateDescriptorJSONUniqueID(id); err != nil {
			t.Errorf("validateDescriptorJSONUniqueID(%q): %v", id, err)
		}
	}
	for _, id := range []string{"", " ", "abc 123", "abc\t123", "abc\x00123", "abc\u2003123", strings.Repeat("a", 129)} {
		if err := validateDescriptorJSONUniqueID(id); err == nil {
			t.Errorf("validateDescriptorJSONUniqueID(%q) succeeded", id)
		}
	}
}

func TestProtectedJSONUniqueIDLogOmitsValue(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(previous)
	logUniqueIDResult("Example", "DPAPI_JSON_SELECT", "must-not-be-logged", nil)
	if strings.Contains(output.String(), "must-not-be-logged") || strings.Contains(output.String(), "valuePreview") {
		t.Fatalf("protected JSON value appeared in logs: %s", output.String())
	}
	if !strings.Contains(output.String(), "read unique id success") {
		t.Fatal("missing success event")
	}
}
