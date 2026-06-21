//go:build windows

package winutil

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tc-hib/winres"
)

// pickFirstIconGroup selects the first RT_GROUP_ICON entry (Explorer / associated-icon semantics).
const pickFirstIconGroup = -1

// ExtractExeIcon writes a PNG of the embedded icon group for exePath to outPNG.
func ExtractExeIcon(exePath, outPNG string) error {
	exePath = filepath.Clean(exePath)
	if st, err := os.Stat(exePath); err != nil || st.IsDir() {
		return fmt.Errorf("invalid exe: %w", err)
	}
	return extractPEIconToPNG(exePath, pickFirstIconGroup, outPNG)
}

func extractPEIconToPNG(pePath string, index int, outPNG string) error {
	pePath = filepath.Clean(pePath)
	if st, err := os.Stat(pePath); err != nil || st.IsDir() {
		return fmt.Errorf("invalid path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outPNG), 0o755); err != nil {
		return err
	}

	f, err := os.Open(pePath)
	if err != nil {
		return err
	}
	defer f.Close()

	rs, err := winres.LoadFromEXESingleType(f, winres.RT_GROUP_ICON)
	if err != nil {
		if errors.Is(err, winres.ErrNoResources) {
			return err
		}
		return err
	}

	gid, err := pickIconGroup(rs, index)
	if err != nil {
		return err
	}

	icon, err := rs.GetIcon(gid)
	if err != nil {
		return fmt.Errorf("get icon: %w", err)
	}

	var buf bytes.Buffer
	if err := icon.SaveICO(&buf); err != nil {
		return err
	}

	img, err := decodeBestFromICO(buf.Bytes())
	if err != nil {
		return fmt.Errorf("decode ico: %w", err)
	}
	return writePNG(outPNG, img)
}

func pickIconGroup(rs *winres.ResourceSet, index int) (winres.Identifier, error) {
	seen := make(map[iconGroupKey]struct{})
	var ids []winres.Identifier
	rs.WalkType(winres.RT_GROUP_ICON, func(resID winres.Identifier, _ uint16, _ []byte) bool {
		k := iconGroupKeyFor(resID)
		if _, ok := seen[k]; ok {
			return true
		}
		seen[k] = struct{}{}
		ids = append(ids, resID)
		return true
	})
	if len(ids) == 0 {
		return nil, fmt.Errorf("no RT_GROUP_ICON resources")
	}

	switch {
	case index == pickFirstIconGroup:
		return ids[0], nil
	case index < -1:
		rid := winres.ID(uint16(-index))
		for _, id := range ids {
			if id == rid {
				return id, nil
			}
		}
		return nil, fmt.Errorf("RT_GROUP_ICON id %d not found", uint16(-index))
	case index >= 0 && index < len(ids):
		return ids[index], nil
	default:
		return nil, fmt.Errorf("icon index %d out of range (have %d groups)", index, len(ids))
	}
}

type iconGroupKey struct {
	isName bool
	id     uint16
	name   string
}

func iconGroupKeyFor(id winres.Identifier) iconGroupKey {
	switch v := id.(type) {
	case winres.ID:
		return iconGroupKey{id: uint16(v)}
	case winres.Name:
		return iconGroupKey{isName: true, name: string(v)}
	default:
		return iconGroupKey{}
	}
}
