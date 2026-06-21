package winutil

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

// Shell link MS-SHLLINK bit flags (subset).
const (
	lnkHasLinkTargetIDList = 0x00000001
	lnkHasLinkInfo         = 0x00000002
	lnkHasName             = 0x00000004
	lnkHasRelativePath     = 0x00000008
	lnkHasWorkingDir       = 0x00000010
	lnkHasArguments        = 0x00000020
	lnkHasIconLocation     = 0x00000040
	lnkIsUnicode           = 0x00000080
)

const (
	linkInfoVolumeIDAndLocalBasePath           = 0x01
	linkInfoCommonNetworkRelativeLinkAndSuffix = 0x02
)

const shellLinkHeaderSize = 76

// parseLnk parses a Windows .lnk file (MS-SHLLINK) and returns TargetPath, Arguments, IconLocation.
func parseLnk(data []byte) (target string, arguments string, iconLocation string, err error) {
	if len(data) < shellLinkHeaderSize {
		return "", "", "", errors.New("lnk: file too short")
	}
	hdrSize := int(binary.LittleEndian.Uint32(data[0:4]))
	if hdrSize < shellLinkHeaderSize || hdrSize > len(data) {
		return "", "", "", fmt.Errorf("lnk: invalid HeaderSize %d", hdrSize)
	}
	linkFlags := binary.LittleEndian.Uint32(data[20:24])

	off := hdrSize
	var linkInfo []byte

	if linkFlags&lnkHasLinkTargetIDList != 0 {
		if off+2 > len(data) {
			return "", "", "", errors.New("lnk: truncated LinkTargetIDList")
		}
		idListSize := int(binary.LittleEndian.Uint16(data[off : off+2]))
		off += 2
		if idListSize < 0 || off+idListSize > len(data) {
			return "", "", "", errors.New("lnk: invalid LinkTargetIDList size")
		}
		off += idListSize
	}

	if linkFlags&lnkHasLinkInfo != 0 {
		if off+4 > len(data) {
			return "", "", "", errors.New("lnk: truncated LinkInfo")
		}
		liSize := int(binary.LittleEndian.Uint32(data[off : off+4]))
		if liSize < 4 || off+liSize > len(data) {
			return "", "", "", errors.New("lnk: invalid LinkInfo size")
		}
		linkInfo = data[off : off+liSize]
		off += liSize
	}

	isUnicode := linkFlags&lnkIsUnicode != 0

	readStr := func() (string, error) {
		if off+2 > len(data) {
			return "", io.ErrUnexpectedEOF
		}
		n := int(binary.LittleEndian.Uint16(data[off : off+2]))
		off += 2
		if n < 0 {
			return "", errors.New("lnk: negative string count")
		}
		if isUnicode {
			byteLen := n * 2
			if off+byteLen > len(data) {
				return "", errors.New("lnk: unicode string out of bounds")
			}
			u := make([]uint16, n)
			for i := 0; i < n; i++ {
				u[i] = binary.LittleEndian.Uint16(data[off+i*2:])
			}
			off += byteLen
			return string(utf16.Decode(u)), nil
		}
		if off+n > len(data) {
			return "", errors.New("lnk: ansi string out of bounds")
		}
		raw := append([]byte(nil), data[off:off+n]...)
		off += n
		s := decodeANSIString(raw)
		return strings.TrimRight(s, "\x00"), nil
	}

	var relPath, workDir string

	if linkFlags&lnkHasName != 0 {
		if _, err = readStr(); err != nil {
			return "", "", "", err
		}
	}
	if linkFlags&lnkHasRelativePath != 0 {
		relPath, err = readStr()
		if err != nil {
			return "", "", "", err
		}
	}
	if linkFlags&lnkHasWorkingDir != 0 {
		workDir, err = readStr()
		if err != nil {
			return "", "", "", err
		}
	}
	if linkFlags&lnkHasArguments != 0 {
		arguments, err = readStr()
		if err != nil {
			return "", "", "", err
		}
	}
	if linkFlags&lnkHasIconLocation != 0 {
		iconLocation, err = readStr()
		if err != nil {
			return "", "", "", err
		}
	}

	if len(linkInfo) > 0 {
		target = pathFromLinkInfo(linkInfo)
	}
	if target == "" && relPath != "" {
		rp := strings.TrimSpace(relPath)
		if filepath.IsAbs(rp) {
			target = filepath.Clean(rp)
		} else if wd := strings.TrimSpace(workDir); wd != "" {
			target = filepath.Clean(filepath.Join(wd, rp))
		} else {
			target = filepath.Clean(rp)
		}
	}

	return strings.TrimSpace(target), strings.TrimSpace(arguments), strings.TrimSpace(iconLocation), nil
}

func pathFromLinkInfo(li []byte) string {
	if len(li) < 28 {
		return ""
	}
	linkInfoSize := int(binary.LittleEndian.Uint32(li[0:4]))
	if linkInfoSize > len(li) {
		linkInfoSize = len(li)
	}
	headerSize := int(binary.LittleEndian.Uint32(li[4:8]))
	if headerSize < 28 || headerSize > linkInfoSize {
		return ""
	}
	flags := binary.LittleEndian.Uint32(li[8:12])

	var base, suf string
	if flags&linkInfoVolumeIDAndLocalBasePath != 0 {
		localOff := binary.LittleEndian.Uint32(li[16:20])
		if localOff != 0 && int(localOff) < linkInfoSize {
			base = readAnsiSZ(li, int(localOff), linkInfoSize)
		}
	}
	if int(headerSize) >= 28 {
		suffixOff := binary.LittleEndian.Uint32(li[24:28])
		if suffixOff != 0 && int(suffixOff) < linkInfoSize {
			suf = readAnsiSZ(li, int(suffixOff), linkInfoSize)
		}
	}

	// Unicode path fields appear in larger headers (prefer when present).
	if int(headerSize) >= 36 {
		// CommonPathSuffixOffsetUnicode at offset 32, LocalBasePathOffsetUnicode at 28 (per MS-SHLLINK)
		baseUOff := binary.LittleEndian.Uint32(li[28:32])
		sufUOff := binary.LittleEndian.Uint32(li[32:36])
		if baseUOff != 0 && int(baseUOff) < linkInfoSize {
			if u := readUTF16Z(li, int(baseUOff), linkInfoSize); u != "" {
				base = u
			}
		}
		if sufUOff != 0 && int(sufUOff) < linkInfoSize {
			if u := readUTF16Z(li, int(sufUOff), linkInfoSize); u != "" {
				suf = u
			}
		}
	}

	base = strings.TrimSpace(base)
	suf = strings.TrimSpace(suf)
	switch {
	case base != "" && suf != "":
		return filepath.Clean(filepath.Join(base, suf))
	case base != "":
		return filepath.Clean(base)
	default:
		return filepath.Clean(suf)
	}
}

func readAnsiSZ(li []byte, off, lim int) string {
	if off < 0 || off >= lim {
		return ""
	}
	end := off
	for end < lim && li[end] != 0 {
		end++
	}
	return decodeANSIString(li[off:end])
}

func readUTF16Z(li []byte, off, lim int) string {
	if off < 0 || off+2 > lim {
		return ""
	}
	var u []uint16
	for i := off; i+1 < lim; i += 2 {
		v := binary.LittleEndian.Uint16(li[i : i+2])
		if v == 0 {
			break
		}
		u = append(u, v)
	}
	if len(u) == 0 {
		return ""
	}
	return string(utf16.Decode(u))
}
