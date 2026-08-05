package winutil

import (
	"encoding/binary"
	"image"
	"testing"
)

// palette8XORIndex is the single palette slot the synthetic icon's colour plane
// uses, so every decoded pixel starts fully opaque and any cleared alpha comes
// from the AND plane rather than from a palette miss.
const palette8XORIndex = 1

var palette8Colour = struct{ R, G, B byte }{R: 0x20, G: 0x40, B: 0x60}

// buildPalette8ICO assembles a one-entry 8bpp ICO whose AND plane is written
// from image-row coordinates. maskRows maps an image row (0 = top) to that
// row's mask byte, MSB first, so a caller states intent in the orientation a
// viewer sees while the builder does the bottom-up file placement.
func buildPalette8ICO(t testing.TB, w, h int, maskRows map[int]byte) []byte {
	t.Helper()
	if w > 8 {
		t.Fatalf("builder writes one mask byte per row, so width must be <= 8, got %d", w)
	}

	const paletteEntries = 2
	xorStride := bmpStride(w, 8)
	andStride := bmpStride(w, 1)
	paletteOffset := bmpHeaderSize
	xorOffset := paletteOffset + paletteEntries*4
	andOffset := xorOffset + xorStride*h
	dib := make([]byte, andOffset+andStride*h)

	binary.LittleEndian.PutUint32(dib[0:4], bmpHeaderSize)
	binary.LittleEndian.PutUint32(dib[4:8], uint32(w))
	// Both stacked planes share one positive biHeight of twice the icon height.
	binary.LittleEndian.PutUint32(dib[8:12], uint32(h*2))
	binary.LittleEndian.PutUint16(dib[12:14], 1)
	binary.LittleEndian.PutUint16(dib[14:16], 8)
	binary.LittleEndian.PutUint32(dib[16:20], biRgb)
	binary.LittleEndian.PutUint32(dib[32:36], paletteEntries)

	// RGBQUAD is stored blue, green, red, reserved.
	entry := paletteOffset + palette8XORIndex*4
	dib[entry] = palette8Colour.B
	dib[entry+1] = palette8Colour.G
	dib[entry+2] = palette8Colour.R

	for row := 0; row < h; row++ {
		for x := 0; x < w; x++ {
			dib[xorOffset+row*xorStride+x] = palette8XORIndex
		}
	}

	for vy, bits := range maskRows {
		if vy < 0 || vy >= h {
			t.Fatalf("mask row %d outside the icon", vy)
		}
		dib[andOffset+(h-1-vy)*andStride] = bits
	}

	const directorySize = 6 + 16
	ico := make([]byte, directorySize+len(dib))
	binary.LittleEndian.PutUint16(ico[2:4], 1)
	binary.LittleEndian.PutUint16(ico[4:6], 1)
	ico[6] = byte(w)
	ico[7] = byte(h)
	ico[8] = paletteEntries
	binary.LittleEndian.PutUint16(ico[10:12], 1)
	binary.LittleEndian.PutUint16(ico[12:14], 8)
	binary.LittleEndian.PutUint32(ico[14:18], uint32(len(dib)))
	binary.LittleEndian.PutUint32(ico[18:22], directorySize)
	copy(ico[directorySize:], dib)
	return ico
}

// TestICOPaletteANDMaskRowOrientation pins the AND plane to the same bottom-up
// row order as the XOR plane. Both planes live in one DIB whose positive
// biHeight means bottom-up storage, so reading the mask top-down mirrors
// transparency vertically.
//
// The icon is deliberately non-square and the mask deliberately asymmetric
// about the horizontal centre line: a square icon, or a mask set on rows that
// mirror onto each other, passes either way.
func TestICOPaletteANDMaskRowOrientation(t *testing.T) {
	const w, h = 4, 6
	// Row 0 fully transparent, row 4 transparent only on its left half. Under a
	// top-down read these land on rows 5 and 1 instead.
	maskRows := map[int]byte{0: 0xF0, 4: 0xC0}

	img, err := decodeBestFromICO(buildPalette8ICO(t, w, h, maskRows))
	if err != nil {
		t.Fatalf("decode synthetic 8bpp icon: %v", err)
	}
	decoded, ok := img.(*image.NRGBA)
	if !ok {
		t.Fatalf("decoded image is %T, want *image.NRGBA", img)
	}
	if got := decoded.Bounds(); got.Dx() != w || got.Dy() != h {
		t.Fatalf("bounds = %v, want %dx%d", got, w, h)
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			pixel := decoded.NRGBAAt(x, y)

			wantAlpha := byte(0xFF)
			if bits, masked := maskRows[y]; masked && bits&(0x80>>uint(x)) != 0 {
				wantAlpha = 0
			}
			if pixel.A != wantAlpha {
				t.Errorf("alpha at (%d,%d) = %d, want %d", x, y, pixel.A, wantAlpha)
			}

			// The mask only clears alpha, so colour must survive everywhere.
			// This also catches a silently mis-parsed palette or XOR plane,
			// which would otherwise let the alpha assertions pass vacuously.
			if pixel.R != palette8Colour.R || pixel.G != palette8Colour.G || pixel.B != palette8Colour.B {
				t.Errorf("colour at (%d,%d) = %v, want %v", x, y, pixel, palette8Colour)
			}
		}
	}
}
