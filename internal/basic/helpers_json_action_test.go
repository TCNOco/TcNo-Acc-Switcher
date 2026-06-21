package basic

import "testing"

func TestParseJSONPathAction_JSONEmptyValue(t *testing.T) {
	filePath, jsonPath, ok := parseJSONPathAction("JSON_EMPTY_VALUE", `JSON_EMPTY_VALUE::%AppData%\Battle.net\Battle.net.config::Client.SavedAccountNames`)
	if !ok {
		t.Fatal("expected parse success")
	}
	if filePath != `%AppData%\Battle.net\Battle.net.config` {
		t.Fatalf("unexpected file path: %q", filePath)
	}
	if jsonPath != "Client.SavedAccountNames" {
		t.Fatalf("unexpected json path: %q", jsonPath)
	}
}

func TestIsJSONEmptyValue(t *testing.T) {
	if !isJSONEmptyValue(`JSON_EMPTY_VALUE::a::b`) {
		t.Fatal("expected JSON_EMPTY_VALUE key to match")
	}
	if isJSONEmptyValue(`JSON_SELECT::a::b`) {
		t.Fatal("did not expect JSON_SELECT key to match JSON_EMPTY_VALUE")
	}
}
