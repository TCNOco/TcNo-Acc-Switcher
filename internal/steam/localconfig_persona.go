package steam

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"TcNo-Acc-Switcher/internal/fsutil"

	"github.com/Jleagle/steam-go/steamid"
	"github.com/Jleagle/steam-go/steamvdf"
	"github.com/tidwall/sjson"
)

var (
	reLeafEPersonaState = regexp.MustCompile(`(?i)"ePersonaState"\s+"(-?\d+)"`)
	// Steam stores FriendStoreLocalPrefs values as JSON with quotes escaped for VDF (e.g. \\\"ePersonaState\\\":1). sjson cannot parse that; replace the number in place.
	reEPSNumberInJSONBlob = regexp.MustCompile(`(?i)(ePersonaState[\\"]*\s*:\s*)(-?\d+)`)
	// escapeVDF doubles every backslash on write; collapse runs so logical values stay single-escaped through KeyValueToText.
	reCollapseBackslashRuns = regexp.MustCompile(`(\\)+`)
)

// setPersonaStateLocalConfig updates persona for the next Steam launch in userdata/<account>/config/localconfig.vdf.
// Steam stores the value under FriendStoreLocalPrefs_* as JSON ({"ePersonaState":n,...}) and sometimes as a plain VDF leaf.
func setPersonaStateLocalConfig(steamRoot, id64 string, ePersonaState int) error {
	sid, err := steamid.ParsePlayerID(id64)
	if err != nil {
		return err
	}
	acc := uint32(sid.GetAccountID())
	path := filepath.Join(steamRoot, "userdata", fmt.Sprintf("%d", acc), "config", "localconfig.vdf")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	raw = bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})

	originalText := string(raw)
	collapsedText := originalText
	if !steamvdf.IsBinary(raw) {
		collapsedText = collapseRepeatedBackslashesStable(originalText)
	}
	rawForParse := []byte(collapsedText)

	kv, err := steamvdf.ReadBytes(rawForParse)
	if err == nil {
		kvRoot := kv
		if kv.Key == "" && len(kv.Children) > 0 {
			kvRoot = kv.Children[0]
		}
		patchedPersona := patchPersonaInKeyValue(&kvRoot, acc, ePersonaState) || ensureFriendStorePrefs(&kvRoot, acc, ePersonaState)
		repairEscaping := collapsedText != originalText
		if patchedPersona || repairEscaping {
			normalizeLocalConfigLeafValues(&kvRoot)
			out := serializeLocalConfigForDisk(kvRoot)
			return fsutil.WriteFileAtomic(path, out, 0o644)
		}
		return nil
	}

	patched := patchPersonaRawString(collapsedText, ePersonaState)
	if patched != collapsedText {
		return fsutil.WriteFileAtomic(path, []byte(dedupeSerializedLocalConfigText(patched)), 0o644)
	}
	return nil
}

func collapseRepeatedBackslashes(s string) string {
	return reCollapseBackslashRuns.ReplaceAllString(s, "\\")
}

// collapseRepeatedBackslashesStable merges every run of backslashes into a single \ (Steam/stacked corruption).
func collapseRepeatedBackslashesStable(s string) string {
	const maxIter = 48
	prev := ""
	for i := 0; i < maxIter && s != prev; i++ {
		prev = s
		s = collapseRepeatedBackslashes(s)
	}
	return s
}

// serializeLocalConfigForDisk builds text VDF then deduplicates backslashes in the final string before bytes hit disk (user-requested).
func serializeLocalConfigForDisk(kvRoot steamvdf.KeyValue) []byte {
	text := string(KeyValueToText(kvRoot))
	return []byte(dedupeSerializedLocalConfigText(text))
}

// dedupeSerializedLocalConfigText collapses every run of backslashes to a single \ in the full file text immediately before save.
func dedupeSerializedLocalConfigText(s string) string {
	return collapseRepeatedBackslashesStable(s)
}

func normalizeLocalConfigLeafValues(kv *steamvdf.KeyValue) {
	var walk func(*steamvdf.KeyValue, bool)
	walk = func(node *steamvdf.KeyValue, underWebStorage bool) {
		for i := range node.Children {
			ch := &node.Children[i]
			inWS := underWebStorage || strings.EqualFold(strings.TrimSpace(ch.Key), "WebStorage")
			if ch.Value != "" {
				ch.Value = normalizeLocalConfigLeafValue(ch.Key, ch.Value, inWS)
			}
			if len(ch.Children) > 0 {
				walk(ch, inWS)
			}
		}
	}
	walk(kv, false)
}

func normalizeLocalConfigLeafValue(key, val string, underWebStorage bool) string {
	val = collapseRepeatedBackslashesStable(val)
	jsonBlob := shouldCanonicalJSONLocalConfigKey(key) ||
		(underWebStorage && strings.HasPrefix(strings.TrimSpace(val), "{"))
	if jsonBlob {
		val = canonicalJSONIfObject(val)
		val = collapseRepeatedBackslashesStable(val)
	}
	return val
}

func shouldCanonicalJSONLocalConfigKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	return strings.HasPrefix(k, "friendstorelocalprefs_") ||
		strings.HasPrefix(k, "friendgroupcollapse_")
}

// canonicalJSONIfObject re-marshals JSON so logical text has no stacked escapes; KeyValueToText/escapeVDF then applies one VDF escape layer.
func canonicalJSONIfObject(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s[0] != '{' {
		return s
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return s
	}
	return string(b)
}

func patchPersonaInKeyValue(kv *steamvdf.KeyValue, accountID uint32, state int) bool {
	changed := false
	var walk func(*steamvdf.KeyValue)
	walk = func(node *steamvdf.KeyValue) {
		for i := range node.Children {
			ch := &node.Children[i]
			if ch.Value != "" {
				if strings.EqualFold(strings.TrimSpace(ch.Key), "ePersonaState") {
					ch.Value = strconv.Itoa(state)
					changed = true
				} else if friendStoreLocalPrefsMatch(ch.Key, accountID) {
					if nv, ok := setPersonaInFriendStoreJSON(ch.Value, state); ok && nv != ch.Value {
						ch.Value = nv
						changed = true
					}
				}
			}
			if len(ch.Children) > 0 {
				walk(ch)
			}
		}
	}
	walk(kv)
	return changed
}

func friendStoreLocalPrefsMatch(key string, accountID uint32) bool {
	key = strings.TrimSpace(key)
	const prefix = "FriendStoreLocalPrefs_"
	lk := strings.ToLower(key)
	lp := strings.ToLower(prefix)
	if !strings.HasPrefix(lk, lp) {
		return false
	}
	suffix := strings.TrimSpace(key[len(prefix):])
	idStr := strconv.FormatUint(uint64(accountID), 10)
	switch {
	case suffix == idStr:
		return true
	case strings.EqualFold(suffix, "["+steamID3Individual(accountID)+"]"):
		return true
	default:
		su := strings.ToUpper(suffix)
		return strings.Contains(su, "[U:1:"+idStr+"]")
	}
}

func steamID3Individual(accountID uint32) string {
	return fmt.Sprintf("U:1:%d", accountID)
}

func setPersonaInFriendStoreJSON(val string, state int) (string, bool) {
	val = strings.TrimSpace(val)
	if val == "" {
		return val, false
	}
	num := strconv.Itoa(state)

	if reEPSNumberInJSONBlob.MatchString(val) {
		out := reEPSNumberInJSONBlob.ReplaceAllString(val, "${1}"+num)
		if out != val {
			return collapseRepeatedBackslashesStable(out), true
		}
		return val, false
	}

	if val[0] == '{' && json.Valid([]byte(val)) {
		out, err := sjson.Set(val, "ePersonaState", state)
		if err != nil {
			return val, false
		}
		if out != val {
			return collapseRepeatedBackslashesStable(out), true
		}
		return val, false
	}

	return val, false
}

func ensureFriendStorePrefs(kv *steamvdf.KeyValue, accountID uint32, state int) bool {
	key := fmt.Sprintf("FriendStoreLocalPrefs_%d", accountID)
	jsonVal := fmt.Sprintf(`{"ePersonaState":%d,"strNonFriendsAllowedToMsg":""}`, state)

	ws := findChildDeep(kv, "WebStorage")
	if ws != nil {
		if friendStorePrefsExists(ws, accountID) {
			return false
		}
		ws.SetChild(steamvdf.KeyValue{Key: key, Value: jsonVal})
		return true
	}

	ul := findChildDeep(kv, "UserLocalConfigStore")
	if ul == nil {
		return false
	}
	ul.SetChild(steamvdf.KeyValue{
		Key:      "WebStorage",
		Children: []steamvdf.KeyValue{{Key: key, Value: jsonVal}},
	})
	return true
}

func friendStorePrefsExists(ws *steamvdf.KeyValue, accountID uint32) bool {
	for _, ch := range ws.Children {
		if friendStoreLocalPrefsMatch(ch.Key, accountID) {
			return true
		}
	}
	return false
}

func findChildDeep(kv *steamvdf.KeyValue, want string) *steamvdf.KeyValue {
	want = strings.ToLower(want)
	var walk func(*steamvdf.KeyValue) *steamvdf.KeyValue
	walk = func(node *steamvdf.KeyValue) *steamvdf.KeyValue {
		for i := range node.Children {
			ch := &node.Children[i]
			if strings.ToLower(ch.Key) == want {
				return ch
			}
			if hit := walk(ch); hit != nil {
				return hit
			}
		}
		return nil
	}
	return walk(kv)
}

func patchPersonaRawString(s string, state int) string {
	out := reLeafEPersonaState.ReplaceAllStringFunc(s, func(_ string) string {
		return fmt.Sprintf(`"ePersonaState"		"%d"`, state)
	})
	num := strconv.Itoa(state)
	out = reEPSNumberInJSONBlob.ReplaceAllString(out, "${1}"+num)
	return out
}
