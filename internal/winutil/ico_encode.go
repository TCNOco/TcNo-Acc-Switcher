package winutil

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// SavePNGsAsICO writes a Windows .ico containing embedded PNG payloads (32bpp), matching Explorer expectations.
func SavePNGsAsICO(w io.Writer, pngs [][]byte) error {
	if len(pngs) == 0 {
		return fmt.Errorf("ico: no images")
	}
	const (
		headerReserved uint16 = 0
		headerIconType uint16 = 1
		headerLength   uint16 = 6
		entryLength    uint16 = 16
		pngPlanes      uint16 = 1
		pngBitCount    uint16 = 32
	)
	dirSize := int(headerLength) + len(pngs)*int(entryLength)
	baseOffset := uint32(dirSize)

	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, headerReserved)
	_ = binary.Write(&buf, binary.LittleEndian, headerIconType)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(len(pngs)))

	offset := baseOffset
	payloads := make([][]byte, 0, len(pngs))
	for _, png := range pngs {
		if len(png) == 0 {
			return fmt.Errorf("ico: empty png payload")
		}
		wb, hb, err := pngDimensions(png)
		if err != nil {
			return fmt.Errorf("ico: %w", err)
		}
		if wb > 256 || hb > 256 {
			return fmt.Errorf("ico: dimensions %dx%d exceed 256", wb, hb)
		}
		var wField, hField byte
		if wb == 256 {
			wField = 0
		} else {
			wField = byte(wb)
		}
		if hb == 256 {
			hField = 0
		} else {
			hField = byte(hb)
		}
		_ = buf.WriteByte(wField)
		_ = buf.WriteByte(hField)
		_ = buf.WriteByte(0) // colors in palette
		_ = buf.WriteByte(0) // reserved
		_ = binary.Write(&buf, binary.LittleEndian, pngPlanes)
		_ = binary.Write(&buf, binary.LittleEndian, pngBitCount)
		sz := uint32(len(png))
		_ = binary.Write(&buf, binary.LittleEndian, sz)
		_ = binary.Write(&buf, binary.LittleEndian, offset)
		offset += sz
		payloads = append(payloads, png)
	}
	for _, p := range payloads {
		if _, err := buf.Write(p); err != nil {
			return err
		}
	}
	_, err := w.Write(buf.Bytes())
	return err
}

func pngDimensions(png []byte) (w, h int, err error) {
	if len(png) < 24 {
		return 0, 0, fmt.Errorf("png too short")
	}
	if png[0] != 0x89 || png[1] != 'P' || png[2] != 'N' || png[3] != 'G' {
		return 0, 0, fmt.Errorf("not a png")
	}
	// IHDR at offset 16: width, height big-endian uint32
	w = int(binary.BigEndian.Uint32(png[16:20]))
	h = int(binary.BigEndian.Uint32(png[20:24]))
	if w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("invalid png dimensions")
	}
	return w, h, nil
}
