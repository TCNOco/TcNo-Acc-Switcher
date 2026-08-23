//go:build !windows

package steam

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"TcNo-Acc-Switcher/internal/fsutil"
	"TcNo-Acc-Switcher/internal/platform"
	"TcNo-Acc-Switcher/internal/vdfsafe"

	"github.com/Jleagle/steam-go/steamvdf"
)

const registryVDFName = "registry.vdf"

// autoLoginKeyPath is where Steam keeps the Windows registry keys the switcher
// writes, inside registry.vdf.
var autoLoginKeyPath = []string{"HKCU", "Software", "Valve", "Steam"}

// writeAutoLogin points Steam at the account to sign in as.
//
// Off Windows there is no registry: Steam keeps the same HKCU\Software\Valve\Steam
// values in a KeyValues file it writes on exit. Which means this is only safe
// once Steam has actually stopped - the switch flow kills it first, and a write
// while it is still running would be overwritten by Steam's own copy on shutdown.
func writeAutoLogin(steamRoot, autoUser string) error {
	path := registryVDFPath(steamRoot)
	if path == "" {
		return fmt.Errorf("could not locate Steam's %s", registryVDFName)
	}
	platform.EmitActionBarStatusI18nVars("Status_UpdatingFile", map[string]string{"file": registryVDFName})

	kv, err := readRegistryVDF(path)
	if err != nil {
		return err
	}
	setRegistryVDFValue(&kv, autoLoginKeyPath, "AutoLoginUser", autoUser)
	setRegistryVDFValue(&kv, autoLoginKeyPath, "RememberPassword", "1")

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, registryVDFText(kv), 0o644)
}

// registryVDFPath locates registry.vdf for a resolved install root.
//
// macOS keeps it beside the rest of the data. Linux keeps it one level out, in
// ~/.steam, next to the symlinks that point back into the install root - so the
// path is derived by stripping the .local/share/Steam tail rather than assuming
// $HOME, which is what makes the Flatpak install under
// ~/.var/app/com.valvesoftware.Steam resolve to its own copy instead of the
// native one.
func registryVDFPath(steamRoot string) string {
	root := strings.TrimSpace(steamRoot)
	if runtime.GOOS == "darwin" {
		if root == "" {
			return ""
		}
		return filepath.Join(filepath.Clean(root), registryVDFName)
	}
	if root != "" {
		root = filepath.Clean(root)
		share := filepath.Dir(root)
		local := filepath.Dir(share)
		if filepath.Base(share) == "share" && filepath.Base(local) == ".local" {
			return filepath.Join(filepath.Dir(local), ".steam", registryVDFName)
		}
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".steam", registryVDFName)
}

// readRegistryVDF returns the parsed file, or an empty tree when there is none
// to read. A file that exists but cannot be parsed is an error: overwriting it
// would throw away whatever else Steam keeps in there.
func readRegistryVDF(path string) (steamvdf.KeyValue, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return steamvdf.KeyValue{Key: "Registry"}, nil
		}
		return steamvdf.KeyValue{}, err
	}
	kv, err := vdfsafe.ReadBytes(raw)
	if err != nil {
		return steamvdf.KeyValue{}, fmt.Errorf("read %s: %w", path, err)
	}
	unescapeRegistryVDF(&kv)
	if strings.TrimSpace(kv.Key) == "" {
		kv.Key = "Registry"
	}
	return kv, nil
}

// setRegistryVDFValue sets key under the named section, creating any part of the
// path that is missing. Section and key names are matched case-insensitively,
// the way Steam's own registry keys are.
func setRegistryVDFValue(root *steamvdf.KeyValue, section []string, key, value string) {
	node := root
	for _, name := range section {
		node = childByNameCI(node, name)
	}
	for i := range node.Children {
		if strings.EqualFold(node.Children[i].Key, key) {
			node.Children[i].Value = value
			node.Children[i].Children = nil
			return
		}
	}
	node.Children = append(node.Children, steamvdf.KeyValue{Key: key, Value: value})
}

func childByNameCI(parent *steamvdf.KeyValue, name string) *steamvdf.KeyValue {
	for i := range parent.Children {
		if strings.EqualFold(parent.Children[i].Key, name) {
			return &parent.Children[i]
		}
	}
	parent.Children = append(parent.Children, steamvdf.KeyValue{Key: name})
	return &parent.Children[len(parent.Children)-1]
}

// registry.vdf belongs to Steam, and a switch rewrites the whole file to change
// two values in it. So reading and writing have to be exactly inverse, or the
// parts we never meant to touch drift further on every switch.
//
// steamvdf's text parser does not unescape. It tracks backslashes only well
// enough to find the closing quote and hands back the raw bytes between them,
// so escaping that again on the way out doubles every backslash - and doubles
// the doubled one next time. Measured on a real install: Steam's
// SourceModInstallPath went from two backslashes to four to eight across two
// switches.
//
// This pair is deliberately local rather than reusing escapeVDF and
// KeyValueToText. Those are the loginusers.vdf path and carry the same
// asymmetry, but they also run on Windows and interact with a deliberate
// localconfig.vdf workaround - not something to change as a side effect.
var registryVDFEscapes = []struct {
	plain   string
	escaped string
}{
	{plain: `\`, escaped: `\\`},
	{plain: `"`, escaped: `\"`},
	{plain: "\n", escaped: `\n`},
	{plain: "\r", escaped: `\r`},
	{plain: "\t", escaped: `\t`},
}

func unescapeRegistryVDF(kv *steamvdf.KeyValue) {
	kv.Key = unescapeRegistryValue(kv.Key)
	kv.Value = unescapeRegistryValue(kv.Value)
	for i := range kv.Children {
		unescapeRegistryVDF(&kv.Children[i])
	}
}

func unescapeRegistryValue(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		matched := false
		for _, e := range registryVDFEscapes {
			if s[i+1] == e.escaped[1] {
				b.WriteString(e.plain)
				i++
				matched = true
				break
			}
		}
		// An escape Valve does not define is left exactly as it was found.
		// Guessing at it would change a value we are only passing through.
		if !matched {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func escapeRegistryValue(s string) string {
	for _, e := range registryVDFEscapes {
		s = strings.ReplaceAll(s, e.plain, e.escaped)
	}
	return s
}

// registryVDFText serialises the tree back to Steam's text format.
//
// Unlike KeyValueToText it keeps a leaf whose value is empty. Steam owns the
// other keys in this file, and silently dropping the ones that happen to be
// blank is an edit to somebody else's data.
func registryVDFText(kv steamvdf.KeyValue) []byte {
	var b strings.Builder
	writeRegistryKV(&b, kv, 0)
	return []byte(b.String())
}

func writeRegistryKV(b *strings.Builder, kv steamvdf.KeyValue, depth int) {
	indent := strings.Repeat("\t", depth)
	if len(kv.Children) == 0 {
		fmt.Fprintf(b, "%s\"%s\"\t\t\"%s\"\n", indent, escapeRegistryValue(kv.Key), escapeRegistryValue(kv.Value))
		return
	}
	fmt.Fprintf(b, "%s\"%s\"\n%s{\n", indent, escapeRegistryValue(kv.Key), indent)
	for _, child := range kv.Children {
		writeRegistryKV(b, child, depth+1)
	}
	fmt.Fprintf(b, "%s}\n", indent)
}
