package profileimage

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// buildAPNG assembles an animated PNG from full-canvas frames, mirroring how
// Steam's avatar frames are laid out: a frame control chunk before IDAT, so the
// default image is also animation frame 0.
func buildAPNG(t *testing.T, frames []*image.NRGBA, offsets []image.Point) []byte {
	t.Helper()
	if len(frames) == 0 {
		t.Fatal("no frames")
	}
	canvas := frames[0].Bounds()

	// Each frame's pixel data is an ordinary PNG stream, so encode and lift the
	// IDAT payloads back out.
	idatOf := func(img *image.NRGBA) ([]byte, []byte) {
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatal(err)
		}
		chunks, err := splitPNGChunks(buf.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		var header, data []byte
		for _, c := range chunks {
			switch c.kind {
			case "IHDR":
				header = c.payload
			case "IDAT":
				data = append(data, c.payload...)
			}
		}
		return header, data
	}

	header, firstData := idatOf(frames[0])
	// The header must describe the whole canvas even when frame 0 does not.
	fullHeader := make([]byte, len(header))
	copy(fullHeader, header)
	binary.BigEndian.PutUint32(fullHeader[0:4], uint32(canvas.Dx()))
	binary.BigEndian.PutUint32(fullHeader[4:8], uint32(canvas.Dy()))

	fcTL := func(seq uint32, img *image.NRGBA, at image.Point) []byte {
		p := make([]byte, 26)
		binary.BigEndian.PutUint32(p[0:4], seq)
		binary.BigEndian.PutUint32(p[4:8], uint32(img.Bounds().Dx()))
		binary.BigEndian.PutUint32(p[8:12], uint32(img.Bounds().Dy()))
		binary.BigEndian.PutUint32(p[12:16], uint32(at.X))
		binary.BigEndian.PutUint32(p[16:20], uint32(at.Y))
		binary.BigEndian.PutUint16(p[20:22], 1)
		binary.BigEndian.PutUint16(p[22:24], 10)
		p[24] = disposeNone
		p[25] = blendSource
		return p
	}

	var out bytes.Buffer
	out.WriteString(pngSignature)
	appendChunk(&out, "IHDR", fullHeader)
	acTL := make([]byte, 8)
	binary.BigEndian.PutUint32(acTL[0:4], uint32(len(frames)))
	appendChunk(&out, "acTL", acTL)

	seq := uint32(0)
	appendChunk(&out, "fcTL", fcTL(seq, frames[0], offsets[0]))
	seq++
	appendChunk(&out, "IDAT", firstData)

	for i := 1; i < len(frames); i++ {
		appendChunk(&out, "fcTL", fcTL(seq, frames[i], offsets[i]))
		seq++
		_, data := idatOf(frames[i])
		fdAT := make([]byte, 4, 4+len(data))
		binary.BigEndian.PutUint32(fdAT[0:4], seq)
		seq++
		fdAT = append(fdAT, data...)
		appendChunk(&out, "fdAT", fdAT)
	}
	appendChunk(&out, "IEND", nil)
	return out.Bytes()
}

// Every test frame keeps some transparency on purpose. Go's PNG encoder drops to
// colour type 2 for a fully opaque image and uses type 6 otherwise, so a fixture
// mixing opaque and transparent frames would hand them different pixel widths
// under one shared IHDR - something a real APNG cannot do, since fdAT frames are
// required to match the header.
func solid(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func decodeStill(t *testing.T, data []byte) *image.NRGBA {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("still did not decode: %v", err)
	}
	out := image.NewNRGBA(img.Bounds())
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			// Through NRGBAModel rather than RGBA(): the latter hands back
			// premultiplied values, which on a faint pixel read as near-black and
			// make an assertion about colour say the opposite of the truth.
			out.Set(x, y, color.NRGBAModel.Convert(img.At(x, y)))
		}
	}
	return out
}

// The whole point: the frame the page could reach on its own is frame 0, and on
// a real border that is the peak of the flash. The still has to be the typical
// frame instead, which is what the eye reads as the border.
func TestStillPicksTheMedianFrameNotTheLoudest(t *testing.T) {
	dim := solid(8, 8, color.NRGBA{10, 10, 10, 40})
	mid := solid(8, 8, color.NRGBA{80, 80, 80, 128})
	blazing := solid(8, 8, color.NRGBA{255, 255, 255, 254})

	data := buildAPNG(t,
		[]*image.NRGBA{blazing, dim, mid, dim, mid},
		[]image.Point{{}, {}, {}, {}, {}})

	still, err := StillFromAPNG(data)
	if err != nil {
		t.Fatalf("StillFromAPNG: %v", err)
	}
	got := decodeStill(t, still)
	if _, _, _, a := got.At(4, 4).RGBA(); uint8(a>>8) != 128 {
		t.Fatalf("still alpha = %d, want the median frame's 128 (frame 0 is 255)", uint8(a>>8))
	}
}

func TestStillCompositesASubRectangleOntoTheCanvas(t *testing.T) {
	// The real frames are mostly sub-rectangles - 202x203 at (11,18) and so on -
	// so a still built from frame data alone, without placing it, would be wrong.
	//
	// The three coverages are deliberately distinct, so the median is the middle
	// frame and the assertions below are about compositing rather than about
	// which frame happened to be picked.
	base := solid(8, 8, color.NRGBA{0, 0, 255, 10})   // faintest
	patch := solid(2, 2, color.NRGBA{255, 0, 0, 250}) // lifts four pixels
	flare := solid(8, 8, color.NRGBA{0, 0, 255, 200}) // brightest

	data := buildAPNG(t,
		[]*image.NRGBA{base, patch, flare},
		[]image.Point{{}, {X: 3, Y: 3}, {}})

	still, err := StillFromAPNG(data)
	if err != nil {
		t.Fatalf("StillFromAPNG: %v", err)
	}
	got := decodeStill(t, still)
	if got.Bounds().Dx() != 8 || got.Bounds().Dy() != 8 {
		t.Fatalf("still is %v, want the full 8x8 canvas", got.Bounds())
	}
	if c := got.NRGBAAt(4, 4); c.R < 200 || c.B != 0 {
		t.Fatalf("patch pixel = %v, want the red sub-rectangle drawn at its offset", c)
	}
	if c := got.NRGBAAt(0, 0); c.B < 200 || c.R != 0 {
		t.Fatalf("background pixel = %v, want the blue frame 0 still underneath", c)
	}
}

func TestStillReportsAPlainPNGAsNotAnimated(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, solid(4, 4, color.NRGBA{1, 2, 3, 255})); err != nil {
		t.Fatal(err)
	}
	if _, err := StillFromAPNG(buf.Bytes()); !errors.Is(err, ErrNotAnimated) {
		t.Fatalf("err = %v, want ErrNotAnimated", err)
	}
}

func TestStillRejectsSomethingThatIsNotAPNG(t *testing.T) {
	if _, err := StillFromAPNG([]byte("GIF89a and then some")); err == nil {
		t.Fatal("expected an error for a non-PNG")
	}
}
