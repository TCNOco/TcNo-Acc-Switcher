package steam

import (
	"fmt"
	"strings"

	"github.com/Jleagle/steam-go/steamvdf"
)

func escapeVDF(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// KeyValueToText serializes a KeyValue tree to Steam text VDF (tab-indented).
func KeyValueToText(kv steamvdf.KeyValue) []byte {
	var b strings.Builder
	writeKV(&b, kv, 0)
	return []byte(b.String())
}

func writeKV(b *strings.Builder, kv steamvdf.KeyValue, depth int) {
	ind := strings.Repeat("\t", depth)
	if len(kv.Children) == 0 {
		if kv.Value != "" {
			fmt.Fprintf(b, "%s\"%s\"\t\t\"%s\"\n", ind, escapeVDF(kv.Key), escapeVDF(kv.Value))
		}
		return
	}
	fmt.Fprintf(b, "%s\"%s\"\n", ind, escapeVDF(kv.Key))
	fmt.Fprintf(b, "%s{\n", ind)
	for _, ch := range kv.Children {
		writeKV(b, ch, depth+1)
	}
	fmt.Fprintf(b, "%s}\n", ind)
}

// LoginUsersToKeyValue builds the "users" subtree from account rows.
func LoginUsersToKeyValue(users []LoginUser) steamvdf.KeyValue {
	var children []steamvdf.KeyValue
	for _, u := range users {
		if strings.TrimSpace(u.SteamID64) == "" {
			continue
		}
		uchild := []steamvdf.KeyValue{
			{Key: "AccountName", Value: u.AccountName},
			{Key: "PersonaName", Value: u.PersonaName},
			{Key: "Timestamp", Value: u.Timestamp},
			{Key: "WantsOfflineMode", Value: u.WantsOffline},
			{Key: "MostRecent", Value: u.MostRecent},
			{Key: "RememberPassword", Value: u.RememberPassword},
		}
		if strings.TrimSpace(u.SkipOfflineWarn) != "" {
			uchild = append(uchild, steamvdf.KeyValue{Key: "SkipOfflineModeWarning", Value: u.SkipOfflineWarn})
		}
		children = append(children, steamvdf.KeyValue{
			Key:      u.SteamID64,
			Children: uchild,
		})
	}
	return steamvdf.KeyValue{Key: "users", Children: children}
}
