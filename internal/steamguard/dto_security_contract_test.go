package steamguard

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode"

	"TcNo-Acc-Switcher/internal/steamguard/confirmationapi"
	"TcNo-Acc-Switcher/internal/steamguard/enrollmentapi"
	"TcNo-Acc-Switcher/internal/steamguard/enrollmentflow"
	"TcNo-Acc-Switcher/internal/steamguard/protocol"
)

var (
	errorType         = reflect.TypeOf((*error)(nil)).Elem()
	jsonMarshalerType = reflect.TypeOf((*json.Marshaler)(nil)).Elem()
)

func TestWailsDTOContractRejectsLongLivedSecrets(t *testing.T) {
	allowed := map[string]string{
		dtoFieldID(reflect.TypeOf(CodeView{}), "Code"):                      "short-lived Steam Guard OTP",
		dtoFieldID(reflect.TypeOf(enrollmentflow.RevocationView{}), "Code"): "single-view revocation recovery code",
		dtoFieldID(reflect.TypeOf(SteamRevocationView{}), "Code"):           "single-view service revocation recovery code",
		dtoFieldID(reflect.TypeOf(SensitiveViewGrant{}), "RequestID"):       "short-lived UI request correlation",
		dtoFieldID(reflect.TypeOf(ConfirmationsGrant{}), "RequestID"):       "short-lived UI request correlation",
	}
	seenAllowed := make(map[string]bool, len(allowed))
	seenTypes := make(map[reflect.Type]bool)

	var inspect func(reflect.Type, string)
	inspect = func(typ reflect.Type, path string) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
			typ = typ.Elem()
		}
		if typ == errorType || typ.Kind() == reflect.Interface || typ.Kind() == reflect.Func || typ.Kind() == reflect.Chan {
			return
		}
		if !strings.HasPrefix(typ.PkgPath(), "TcNo-Acc-Switcher/internal/steamguard") {
			return
		}
		if secretShapedType(typ) {
			t.Errorf("%s exposes forbidden secret-shaped type %s", path, typ)
		}
		if typ.Kind() != reflect.Struct || seenTypes[typ] {
			return
		}
		seenTypes[typ] = true
		if typ.Implements(jsonMarshalerType) || reflect.PointerTo(typ).Implements(jsonMarshalerType) {
			t.Errorf("%s (%s) has custom JSON serialization that requires an explicit contract review", path, typ)
		}

		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if !field.IsExported() || jsonFieldName(field) == "-" {
				continue
			}
			fieldPath := path + "." + field.Name
			if secretShapedDTOField(field) {
				id := dtoFieldID(typ, field.Name)
				if _, ok := allowed[id]; !ok {
					t.Errorf("%s (%s) can serialize forbidden secret-shaped field %q", fieldPath, field.Type, jsonFieldName(field))
				} else {
					seenAllowed[id] = true
				}
			}
			inspect(field.Type, fieldPath)
		}
	}

	serviceType := reflect.TypeOf((*Service)(nil))
	if serviceType.NumMethod() == 0 {
		t.Fatal("Steam Guard service exposes no methods to inspect")
	}
	for i := 0; i < serviceType.NumMethod(); i++ {
		method := serviceType.Method(i)
		for input := 1; input < method.Type.NumIn(); input++ {
			inspect(method.Type.In(input), "Service."+method.Name+".request")
		}
		for output := 0; output < method.Type.NumOut(); output++ {
			inspect(method.Type.Out(output), "Service."+method.Name+".response")
		}
	}

	// These event payloads and the reviewed enrollment views cross the same Go to
	// WebView boundary even though they are not direct Service method results.
	inspect(reflect.TypeOf(SensitiveViewGrant{}), "event.SensitiveViewGrant")
	inspect(reflect.TypeOf(ConfirmationsGrant{}), "event.ConfirmationsGrant")
	inspect(reflect.TypeOf(enrollmentflow.Status{}), "enrollment.Status")
	inspect(reflect.TypeOf(enrollmentflow.RevocationView{}), "enrollment.RevocationView")

	for id, reason := range allowed {
		if !seenAllowed[id] {
			t.Errorf("reviewed DTO exception %s (%s) is stale or no longer inspected", id, reason)
		}
	}
	if len(seenTypes) < 10 {
		t.Fatalf("inspected only %d Steam Guard DTO types; reflection root set is unexpectedly narrow", len(seenTypes))
	}
}

func TestReviewedSecretViewsSerializeOnlyTheirBoundedValue(t *testing.T) {
	const marker = "dto-secret-contract-marker"
	assertJSON := func(t *testing.T, value any, wantKey string) {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		if fields[wantKey] != marker || strings.Count(string(raw), marker) != 1 {
			t.Fatalf("reviewed secret view serialized unexpected shape: %s", raw)
		}
	}

	assertJSON(t, CodeView{Code: marker}, "code")
	assertJSON(t, enrollmentflow.RevocationView{Code: marker}, "code")
	assertJSON(t, SteamRevocationView{Code: marker}, "code")
}

func TestSecretBearingOperationsAndValuesRedactErrorsAndFormatting(t *testing.T) {
	const (
		passwordMarker = "password-DONT-LEAK-4f9094"
		tokenMarker    = "token-DONT-LEAK-e4f2b3"
		secretMarker   = "secret-DONT-LEAK-7813ca"
		accountMarker  = "account-DONT-LEAK-15d7a2"
	)
	markers := []string{passwordMarker, tokenMarker, secretMarker, accountMarker}

	auth := protocol.NewAuthenticationClient(nil)
	_, authErr := auth.BeginAuthSessionWithPassword(context.Background(), protocol.PasswordCredentialsRequest{
		AccountName: accountMarker,
	}, []byte(passwordMarker), time.Second)
	assertRedactedError(t, "protocol authentication", authErr, markers)

	confirmations := confirmationapi.NewClient(confirmationapi.Options{Offline: func() bool { return true }})
	credentials := confirmationapi.Credentials{
		SteamID: accountMarker, DeviceID: secretMarker, IdentitySecret: secretMarker,
		AccessToken: tokenMarker, SessionID: secretMarker,
	}
	_, confirmationErr := confirmations.List(context.Background(), credentials)
	assertRedactedError(t, "confirmation operation", confirmationErr, markers)
	assertRedactedFormatting(t, "confirmation credentials", credentials, markers)

	manager := &enrollmentflow.Manager{}
	_, enrollmentErr := manager.Start(context.Background(), enrollmentflow.StartRequest{
		SteamID: 76561198000000000, AccessToken: []byte(tokenMarker), AuthenticatorTime: 1,
	})
	assertRedactedError(t, "enrollment operation", enrollmentErr, markers)

	pending := &enrollmentapi.PendingEnrollment{
		AccessToken: []byte(tokenMarker), SharedSecret: []byte(secretMarker),
		IdentitySecret: []byte(secretMarker), RevocationCode: []byte(secretMarker),
		AccountName: accountMarker,
	}
	assertRedactedFormatting(t, "pending enrollment", pending, markers)

	revocation := &enrollmentflow.RevocationView{Code: secretMarker}
	assertRedactedFormatting(t, "revocation view", revocation, markers)
	serviceRevocation := &SteamRevocationView{Code: secretMarker}
	assertRedactedFormatting(t, "service revocation view", serviceRevocation, markers)

	useSettingsRoot(t)
	_, serviceErr := (&Service{}).UnlockAccount(accountMarker, passwordMarker, false, tokenMarker)
	assertRedactedError(t, "Steam Guard service", serviceErr, markers)
}

func assertRedactedError(t *testing.T, operation string, err error, markers []string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s unexpectedly succeeded", operation)
	}
	assertNoMarkers(t, operation+" error", err.Error(), markers)
	assertNoMarkers(t, operation+" wrapped error", fmt.Errorf("operation failed: %w", err).Error(), markers)
}

func assertRedactedFormatting(t *testing.T, name string, value any, markers []string) {
	t.Helper()
	for _, format := range []string{"%v", "%+v", "%#v", "%s"} {
		assertNoMarkers(t, name+" "+format, fmt.Sprintf(format, value), markers)
	}
}

func assertNoMarkers(t *testing.T, name, rendered string, markers []string) {
	t.Helper()
	for _, marker := range markers {
		if strings.Contains(rendered, marker) {
			t.Errorf("%s exposed sentinel %q in %q", name, marker, rendered)
		}
	}
}

func dtoFieldID(owner reflect.Type, field string) string {
	return owner.PkgPath() + "." + owner.Name() + "." + field
}

func jsonFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if comma := strings.IndexByte(tag, ','); comma >= 0 {
		tag = tag[:comma]
	}
	if tag != "" {
		return tag
	}
	return field.Name
}

func secretShapedDTOField(field reflect.StructField) bool {
	if !canCarrySecret(field.Type) {
		return false
	}
	name := normalizedIdentifier(field.Name + " " + jsonFieldName(field) + " " + field.Type.Name())
	for _, forbidden := range []string{
		"password", "passphrase", "secret", "token", "sessionid", "cookie",
		"authorization", "guarddata", "requestid", "revocationcode", "encryptedpassword",
		"confirmationcode", "otpcode", "steamguardcode",
	} {
		if strings.Contains(name, forbidden) {
			return true
		}
	}
	name = normalizedIdentifier(field.Name)
	return name == "uri" || name == "code"
}

func canCarrySecret(typ reflect.Type) bool {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() == reflect.String || typ.Kind() == reflect.Interface {
		return true
	}
	switch typ.Kind() {
	case reflect.Slice, reflect.Array:
		return typ.Elem().Kind() == reflect.Uint8 || canCarrySecret(typ.Elem())
	case reflect.Map:
		return canCarrySecret(typ.Elem())
	default:
		return typ.Implements(jsonMarshalerType) || (typ.Kind() != reflect.Pointer && reflect.PointerTo(typ).Implements(jsonMarshalerType))
	}
}

func secretShapedType(typ reflect.Type) bool {
	name := normalizedIdentifier(typ.Name())
	for _, forbidden := range []string{
		"password", "passphrase", "secret", "accesstoken", "refreshtoken",
		"sessiontoken", "sessioncookie", "authorization", "revocationcode",
	} {
		if strings.Contains(name, forbidden) {
			return true
		}
	}
	return false
}

func normalizedIdentifier(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, value)
}
