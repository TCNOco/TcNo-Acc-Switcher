package qrimage

import (
	"context"
	"image"
	"image/color"
	"math"
	"testing"
)

// photographed degrades a clean symbol the way a phone camera pointed at a
// screen does: fewer pixels per module, grey-on-grey instead of black-on-white,
// and the softness of a lens that never quite focused.
//
// The numbers come from a real photo that this package could not read - its
// luminance ran 99..255 rather than 0..255, at about four and a half pixels per
// module.
func photographed(t testing.TB, source *Frame) *Frame {
	t.Helper()
	src := source.Image()
	bounds := src.Bounds()
	const shrink = 0.75
	width := int(float64(bounds.Dx()) * shrink)
	height := int(float64(bounds.Dy()) * shrink)

	const (
		darkLevel  = 99
		lightLevel = 250
	)
	gray := image.NewGray(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Bilinear, because a camera integrates over its sensor cells rather
			// than picking one source pixel.
			fx := (float64(x) + 0.5) / shrink
			fy := (float64(y) + 0.5) / shrink
			x0, y0 := int(fx-0.5), int(fy-0.5)
			dx, dy := fx-0.5-float64(x0), fy-0.5-float64(y0)
			at := func(px, py int) float64 {
				px = min(max(px, 0), bounds.Dx()-1)
				py = min(max(py, 0), bounds.Dy()-1)
				r, g, b, _ := src.At(bounds.Min.X+px, bounds.Min.Y+py).RGBA()
				return float64(299*int(r>>8)+587*int(g>>8)+114*int(b>>8)) / 1000
			}
			v := at(x0, y0)*(1-dx)*(1-dy) + at(x0+1, y0)*dx*(1-dy) +
				at(x0, y0+1)*(1-dx)*dy + at(x0+1, y0+1)*dx*dy
			squashed := darkLevel + v*(lightLevel-darkLevel)/255
			gray.SetGray(x, y, color.Gray{Y: uint8(math.Round(squashed))})
		}
	}

	frame := &Frame{Width: width, Height: height, Stride: width * 4, Pixels: make([]byte, width*height*4)}
	out := frame.Image()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var sum, n int
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if nx < 0 || ny < 0 || nx >= width || ny >= height {
						continue
					}
					sum += int(gray.GrayAt(nx, ny).Y)
					n++
				}
			}
			v := uint8(sum / n)
			out.SetNRGBA(x, y, color.NRGBA{R: v, G: v, B: v, A: 0xff})
		}
	}
	return frame
}

// A photo of a sign-in code on a screen is a thing people actually try, and the
// decoder this package used before could not read one: it located the finder
// patterns and then failed Reed-Solomon, because it fits the module grid with an
// affine transform and a photograph is not an affine view of a screen.
func TestDecodeCandidatesReadsAPhotographedCode(t *testing.T) {
	payload := "https://s.team/q/1/1234567890123456789"
	frame := photographed(t, makeQRFrame(t, payload))

	candidates, err := DecodeCandidates(context.Background(), frame)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].Payload != payload {
		t.Fatalf("candidates = %#v, want the challenge", candidates)
	}
}
