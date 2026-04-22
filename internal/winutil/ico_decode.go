package winutil

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

const (
	bmpHeaderSize      = 40 // BITMAPINFOHEADER
	biRgb              = 0
	biBitfields        = 3
	biPng              = 5
	pngMagicLen        = 8
	pngMagic0          = 0x89
	maxICOImageBytes   = 10 << 20
	maxICOEntryCount   = 512
)

var pngMagic = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}

// decodeBestFromICO picks the largest / highest bit-depth image from an ICO byte stream and decodes it.
func decodeBestFromICO(data []byte) (image.Image, error) {
	if len(data) < 6 {
		return nil, errors.New("ico: file too short")
	}
	if data[0] != 0 || data[1] != 0 {
		return nil, errors.New("ico: invalid reserved field")
	}
	if binary.LittleEndian.Uint16(data[2:4]) != 1 {
		return nil, errors.New("ico: not an icon image type")
	}
	n := int(binary.LittleEndian.Uint16(data[4:6]))
	if n <= 0 || n > maxICOEntryCount {
		return nil, fmt.Errorf("ico: invalid entry count %d", n)
	}
	if len(data) < 6+n*16 {
		return nil, errors.New("ico: truncated directory")
	}

	best := -1
	bestScore := -1
	for i := 0; i < n; i++ {
		base := 6 + i*16
		w := int(data[base])
		if w == 0 {
			w = 256
		}
		h := int(data[base+1])
		if h == 0 {
			h = 256
		}
		bpp := int(binary.LittleEndian.Uint16(data[base+6 : base+8]))
		sz := int(binary.LittleEndian.Uint32(data[base+8 : base+12]))
		off := int(binary.LittleEndian.Uint32(data[base+12 : base+16]))
		if sz <= 0 || sz > maxICOImageBytes || off < 0 || off+sz > len(data) {
			continue
		}
		isPNG := sz >= pngMagicLen && off+pngMagicLen <= len(data) && bytes.HasPrefix(data[off:off+pngMagicLen], pngMagic)
		score := w*h*10000 + bpp*100
		if isPNG {
			score += 50 // tie-break toward PNG payloads when dimensions match
		}
		if score > bestScore {
			bestScore = score
			best = i
		}
	}
	if best < 0 {
		return nil, errors.New("ico: no decodable entries")
	}
	base := 6 + best*16
	sz := int(binary.LittleEndian.Uint32(data[base+8 : base+12]))
	off := int(binary.LittleEndian.Uint32(data[base+12 : base+16]))
	return decodeICOEntry(data[off : off+sz])
}

func decodeICOEntry(payload []byte) (image.Image, error) {
	if len(payload) >= pngMagicLen && bytes.HasPrefix(payload, pngMagic) {
		return png.Decode(bytes.NewReader(payload))
	}
	return decodeIconDIB(payload)
}

// writePNG writes img as PNG, creating parent directories as needed.
func writePNG(outPath string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func decodeIconDIB(data []byte) (image.Image, error) {
	if len(data) < bmpHeaderSize {
		return nil, errors.New("ico dib: bitmap header too short")
	}
	biSize := binary.LittleEndian.Uint32(data[0:4])
	if biSize < bmpHeaderSize || int(biSize) > len(data) {
		return nil, errors.New("ico dib: invalid biSize")
	}
	width := int(int32(binary.LittleEndian.Uint32(data[4:8])))
	height := int(int32(binary.LittleEndian.Uint32(data[8:12])))
	if width <= 0 || height <= 0 {
		return nil, errors.New("ico dib: invalid dimensions")
	}
	if height%2 != 0 {
		return nil, errors.New("ico dib: odd biHeight")
	}
	xorHeight := height / 2
	planes := binary.LittleEndian.Uint16(data[12:14])
	bpp := binary.LittleEndian.Uint16(data[14:16])
	compression := binary.LittleEndian.Uint32(data[16:20])
	clrUsed := binary.LittleEndian.Uint32(data[32:36])
	if planes != 1 {
		return nil, fmt.Errorf("ico dib: unsupported planes %d", planes)
	}

	off := int(biSize)
	if compression == biPng {
		if off >= len(data) {
			return nil, errors.New("ico dib: BI_PNG without payload")
		}
		return png.Decode(bytes.NewReader(data[off:]))
	}
	if compression != biRgb && compression != biBitfields {
		return nil, fmt.Errorf("ico dib: unsupported compression %d", compression)
	}

	switch bpp {
	case 32, 24, 8, 4, 1:
	default:
		return nil, fmt.Errorf("ico dib: unsupported bit count %d", bpp)
	}

	nPalette := int(clrUsed)
	if nPalette == 0 && bpp < 16 {
		nPalette = 1 << bpp
	}
	var palette []color.RGBA
	if bpp <= 8 {
		if off+nPalette*4 > len(data) {
			return nil, errors.New("ico dib: truncated palette")
		}
		palette = make([]color.RGBA, nPalette)
		for i := 0; i < nPalette; i++ {
			b := data[off+i*4]
			g := data[off+i*4+1]
			r := data[off+i*4+2]
			palette[i] = color.RGBA{R: r, G: g, B: b, A: 255}
		}
		off += nPalette * 4
	}

	xorStride := bmpStride(width, int(bpp))
	xorSize := xorStride * xorHeight
	andStride := bmpStride(width, 1)
	andSize := andStride * xorHeight
	if off+xorSize+andSize > len(data) {
		return nil, errors.New("ico dib: truncated xor/and planes")
	}

	xorOff := off
	andOff := off + xorSize

	img := image.NewNRGBA(image.Rect(0, 0, width, xorHeight))

	switch bpp {
	case 32:
		decodeXOR32(img, data, xorOff, xorStride, width, xorHeight)
	case 24:
		decodeXOR24(img, data, xorOff, xorStride, width, xorHeight)
	case 8:
		decodeXOR8(img, data, xorOff, xorStride, width, xorHeight, palette)
	case 4:
		decodeXOR4(img, data, xorOff, xorStride, width, xorHeight, palette)
	case 1:
		decodeXOR1(img, data, xorOff, xorStride, width, xorHeight, palette)
	}

	applyANDMask(img, data, andOff, andStride, width, xorHeight)

	return img, nil
}

func bmpStride(width, bpp int) int {
	return ((width*bpp + 31) / 32) * 4
}

func decodeXOR32(img *image.NRGBA, data []byte, base, stride, w, h int) {
	for vy := 0; vy < h; vy++ {
		bmpRow := h - 1 - vy
		row := base + bmpRow*stride
		for vx := 0; vx < w; vx++ {
			o := row + vx*4
			b := data[o]
			g := data[o+1]
			r := data[o+2]
			a := data[o+3]
			img.SetNRGBA(vx, vy, color.NRGBA{R: r, G: g, B: b, A: a})
		}
	}
}

func decodeXOR24(img *image.NRGBA, data []byte, base, stride, w, h int) {
	for vy := 0; vy < h; vy++ {
		bmpRow := h - 1 - vy
		row := base + bmpRow*stride
		for vx := 0; vx < w; vx++ {
			o := row + vx*3
			b := data[o]
			g := data[o+1]
			r := data[o+2]
			img.SetNRGBA(vx, vy, color.NRGBA{R: r, G: g, B: b, A: 255})
		}
	}
}

func decodeXOR8(img *image.NRGBA, data []byte, base, stride, w, h int, pal []color.RGBA) {
	for vy := 0; vy < h; vy++ {
		bmpRow := h - 1 - vy
		row := base + bmpRow*stride
		for vx := 0; vx < w; vx++ {
			idx := data[row+vx]
			if int(idx) >= len(pal) {
				continue
			}
			c := pal[idx]
			img.SetNRGBA(vx, vy, color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A})
		}
	}
}

func decodeXOR4(img *image.NRGBA, data []byte, base, stride, w, h int, pal []color.RGBA) {
	for vy := 0; vy < h; vy++ {
		bmpRow := h - 1 - vy
		row := base + bmpRow*stride
		for vx := 0; vx < w; vx++ {
			b := data[row+vx/2]
			var idx byte
			if vx%2 == 0 {
				idx = b >> 4
			} else {
				idx = b & 0x0f
			}
			if int(idx) >= len(pal) {
				continue
			}
			c := pal[idx]
			img.SetNRGBA(vx, vy, color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A})
		}
	}
}

func decodeXOR1(img *image.NRGBA, data []byte, base, stride, w, h int, pal []color.RGBA) {
	for vy := 0; vy < h; vy++ {
		bmpRow := h - 1 - vy
		row := base + bmpRow*stride
		for vx := 0; vx < w; vx++ {
			b := data[row+vx/8]
			bit := 7 - uint(vx%8)
			idx := (b >> bit) & 1
			if int(idx) >= len(pal) {
				continue
			}
			c := pal[idx]
			img.SetNRGBA(vx, vy, color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A})
		}
	}
}

func applyANDMask(img *image.NRGBA, data []byte, base, stride, w, h int) {
	for vy := 0; vy < h; vy++ {
		row := base + vy*stride
		for vx := 0; vx < w; vx++ {
			bit := uint(vx % 8)
			b := data[row+vx/8]
			andSet := (b>>(7-bit))&1 != 0
			if !andSet {
				continue
			}
			c := img.NRGBAAt(vx, vy)
			c.A = 0
			img.SetNRGBA(vx, vy, c)
		}
	}
}

// decodeANSIString converts legacy ANSI shortcut bytes using Windows-1252 when UTF-8 validation fails.
func decodeANSIString(b []byte) string {
	s := string(bytes.TrimSuffix(b, []byte{0}))
	if s == "" {
		return ""
	}
	if utf8.ValidString(s) {
		return s
	}
	dec := charmap.Windows1252.NewDecoder()
	out, err := dec.Bytes([]byte(s))
	if err != nil {
		return s
	}
	return string(out)
}
