package qrimage

import (
	"context"
	"errors"
	"image"
	"image/color"
	"math"
	"testing"
	"time"
)

// photographed degrades a clean symbol the way a phone camera pointed at a
// screen does: fewer pixels per module, grey-on-grey instead of black-on-white,
// and the softness of a lens that never quite focused. The levels match a real
// photo whose luminance ran 99..255 rather than 0..255, at about four and a half
// pixels per module.
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

// A photo of a sign-in code on a screen is a thing people actually try.
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

// A screenshot decodes as it arrives and must not pay for thresholding it never
// needed.
func TestDecodeCandidatesOnlyRedrawsWhenTheFrameItselfHeldNothing(t *testing.T) {
	payload := "https://s.team/q/1/1234567890123456789"

	var reads int
	readable := func(image.Image) (string, error) {
		reads++
		return payload, nil
	}
	if _, err := decodeCandidates(context.Background(), []*Frame{makeQRFrame(t, payload)},
		readable, time.Now, MaxCandidateFrames, MaxCandidatePixels, MaxCandidateDecodeTime); err != nil {
		t.Fatal(err)
	}
	readsWhenFound := reads

	reads = 0
	blind := func(image.Image) (string, error) {
		reads++
		return "", errors.New("no code here")
	}
	if _, err := decodeCandidates(context.Background(), []*Frame{makeQRFrame(t, payload)},
		blind, time.Now, MaxCandidateFrames, MaxCandidatePixels, MaxCandidateDecodeTime); err != nil {
		t.Fatal(err)
	}
	if reads <= readsWhenFound+1 {
		t.Fatalf("reads = %d after a frame that held nothing, %d when it decoded: the redraw did not run",
			reads, readsWhenFound)
	}
}

// A variant that still carried the photograph's greys would only be asking the
// decoder the same question twice.
func TestNormalizedVariantsAreBinaryAndTheSameSize(t *testing.T) {
	frame := photographed(t, makeQRFrame(t, "https://s.team/q/1/1234567890123456789"))
	source := frame.Image()

	variants := normalizedVariants(source)
	if len(variants) != maxNormalizedVariants {
		t.Fatalf("variants = %d, want %d", len(variants), maxNormalizedVariants)
	}
	for index, variant := range variants {
		if variant.Bounds().Dx() != source.Bounds().Dx() || variant.Bounds().Dy() != source.Bounds().Dy() {
			t.Fatalf("variant %d is %v, want %v", index, variant.Bounds(), source.Bounds())
		}
		gray, ok := variant.(*image.Gray)
		if !ok {
			t.Fatalf("variant %d is %T", index, variant)
		}
		for _, value := range gray.Pix {
			if value != 0 && value != 255 {
				t.Fatalf("variant %d still holds grey (%d), so it was never thresholded", index, value)
			}
		}
	}

	wipeVariants(variants)
	for index, variant := range variants {
		for _, value := range variant.(*image.Gray).Pix {
			if value != 0 {
				t.Fatalf("variant %d survived the wipe", index)
			}
		}
	}
}

func TestNormalizedVariantsDeclineAnImageTooSmallToHoldACode(t *testing.T) {
	tiny := image.NewNRGBA(image.Rect(0, 0, minNormalizedDimension-1, minNormalizedDimension-1))
	if variants := normalizedVariants(tiny); variants != nil {
		t.Fatalf("variants = %d for an image no code fits in", len(variants))
	}
}
