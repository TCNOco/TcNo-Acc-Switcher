package basic

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tidwall/gjson"
)

func TestBattleNetLegacyAccountsSurviveSaveAndRestore(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	fc.PlatformKey = "BattleNet"
	config := filepath.Join(env.instDir, "Battle.net.config")
	fc.Descriptor.UniqueIdMethod = "JSON_SELECT_FIRST,"
	fc.Descriptor.UniqueIdFile = config + "::Client.SavedAccountNames"
	fc.Descriptor.LoginFiles = map[string]string{
		"JSON_SELECT_FIRST,::" + config + "::Client.SavedAccountNames": "email.json",
	}
	index, err := idsPath(fc.PlatformKey)
	if err != nil {
		t.Fatal(err)
	}
	legacy := `{"first@example.test":"First#1234","second@example.test":"Second#5678"}`
	mustWrite(t, index, legacy)
	for email, name := range map[string]string{"first@example.test": "First#1234", "second@example.test": "Second#5678"} {
		base, err := loginCacheRoot(fc.PlatformKey)
		if err != nil {
			t.Fatal(err)
		}
		// Use the actual .NET folder name, before v4 normalization.
		mustWrite(t, filepath.Join(base, name, "email.json"), `"`+email+`"`)
	}
	order := []string{"second@example.test", "first@example.test"}
	if err := writeOrder(fc.PlatformKey, order); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, config, `{"Client":{"SavedAccountNames":"third@example.test,first@example.test"},"Keep":true}`)
	// Saving a new account must migrate before updating the index, even if
	// the user has not visited the account page after upgrading.
	if err := saveCurrentAfterKill(FlowDeps{}, "Third#9012", fc); err != nil {
		t.Fatal(err)
	}
	ids, err := readIDs(fc.PlatformKey)
	want := map[string]string{"first@example.test": "First#1234", "second@example.test": "Second#5678", "third@example.test": "Third_9012"}
	if err != nil || !reflect.DeepEqual(ids, want) {
		t.Fatalf("IDs = %v, %v", ids, err)
	}
	if got := mustRead(t, index+".legacy-dotnet.bak"); got != legacy {
		t.Fatalf("legacy backup = %s", got)
	}
	if got, err := readOrder(fc.PlatformKey); err != nil || !reflect.DeepEqual(got, order) {
		t.Fatalf("order = %v, %v", got, err)
	}
	mustWrite(t, config, `{"Client":{"SavedAccountNames":"fourth@example.test"},"Keep":true}`)
	if err := saveCurrentAfterKill(FlowDeps{}, "First#1234", fc); err == nil {
		t.Fatal("new account overwrote a legacy account with the same display name")
	}
	for email, name := range want {
		if err := Login(FlowDeps{}, fc, name); err != nil {
			t.Fatal(err)
		}
		if got := gjson.Get(mustRead(t, config), "Client.SavedAccountNames").String(); got != email {
			t.Fatalf("restored %s as %q", name, got)
		}
		if !gjson.Get(mustRead(t, config), "Keep").Bool() {
			t.Fatal("restore replaced unrelated launcher settings")
		}
	}
}

func TestSaveRejectsMissingIdentityWithoutChangingLegacyAccounts(t *testing.T) {
	env := newFlowTestEnv(t)
	fc := env.flowContext()
	config := filepath.Join(env.instDir, "Battle.net.config")
	fc.Descriptor.UniqueIdMethod = "JSON_SELECT_FIRST,"
	fc.Descriptor.UniqueIdFile = config + "::Client.SavedAccountNames"
	index, err := idsPath(fc.PlatformKey)
	if err != nil {
		t.Fatal(err)
	}
	legacy := `{"old@example.test":"Old Account"}`
	mustWrite(t, index, legacy)
	cache := filepath.Join(env.cacheDir("Old Account"), "Saved", "config.cfg")
	mustWrite(t, cache, "original saved session")
	mustWrite(t, config, `{"Client":{"SavedAccountNames":""}}`)
	if err := saveCurrentAfterKill(FlowDeps{}, "Old Account", fc); err == nil {
		t.Fatal("save without an identity succeeded")
	}
	if got := mustRead(t, index); got != legacy {
		t.Fatalf("index changed: %s", got)
	}
	if got := mustRead(t, cache); got != "original saved session" {
		t.Fatalf("saved account changed: %s", got)
	}
}
