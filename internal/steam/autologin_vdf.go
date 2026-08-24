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

var autoLoginKeyPath = []string{"HKCU", "Software", "Valve", "Steam"}

// writeAutoLogin points Steam at the account to sign in as. Off Windows there is
// no registry: Steam keeps the same HKCU\Software\Valve\Steam values in a
// KeyValues file it rewrites on exit, so this is only safe once Steam has
// actually stopped - which the switch flow guarantees by killing it first.
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

// registryVDFPath locates registry.vdf for a resolved install root. macOS keeps
// it beside the data; Linux keeps it in ~/.steam, so the path is derived by
// stripping the .local/share/Steam tail rather than assuming $HOME - which is
// what makes a Flatpak install resolve to its own copy and not the native one.
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

// readRegistryVDF returns an empty tree when there is no file. A file that
// exists but cannot be parsed is an error: overwriting it would throw away
// whatever else Steam keeps in there.
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
	if strings.TrimSpace(kv.Key) == "" {
		kv.Key = "Registry"
	}
	return kv, nil
}

// setRegistryVDFValue sets key under the named section, creating any missing
// part of the path. Names match case-insensitively, as Steam's own keys do.
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

// registryVDFText serialises the tree back to text VDF. Unlike KeyValueToText it
// keeps a leaf whose value is empty: this file is Steam's, and dropping the keys
// that happen to be blank is an edit to somebody else's data.
func registryVDFText(kv steamvdf.KeyValue) []byte {
	var b strings.Builder
	writeRegistryKV(&b, kv, 0)
	return []byte(b.String())
}

func writeRegistryKV(b *strings.Builder, kv steamvdf.KeyValue, depth int) {
	indent := strings.Repeat("\t", depth)
	if len(kv.Children) == 0 {
		fmt.Fprintf(b, "%s\"%s\"\t\t\"%s\"\n", indent, vdfsafe.Escape(kv.Key), vdfsafe.Escape(kv.Value))
		return
	}
	fmt.Fprintf(b, "%s\"%s\"\n%s{\n", indent, vdfsafe.Escape(kv.Key), indent)
	for _, child := range kv.Children {
		writeRegistryKV(b, child, depth+1)
	}
	fmt.Fprintf(b, "%s}\n", indent)
}
