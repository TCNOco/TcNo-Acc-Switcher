package qrimage

import (
	"image"
	"image/color"
	"math"
	"runtime"
)

// Bounds on the second-attempt pass. Each variant is one extra whole-frame
// decode, not another set of regions: this exists for an image that is one
// photograph of one code, where splitting the frame was never the problem.
const (
	maxNormalizedVariants = 2
	// minNormalizedDimension is below the smallest QR symbol plus its quiet
	// zone, so nothing is redrawn that could not have held a code anyway.
	minNormalizedDimension = 21
	// unsharpRadius and unsharpAmount put back the module edges a camera that
	// never quite focused rounded off. Measured on photographs of a Steam
	// sign-in code: a one-pixel radius at 1.5 recovered codes that no threshold
	// alone could, and heavier settings started inventing edges of their own.
	unsharpRadius = 1
	unsharpAmount = 1.5
)

// normalizedVariants re-draws a frame for a second decode attempt.
//
// A screenshot needs none of this, and does not get it: the variants are only
// built after the frame itself has been read and found to hold nothing. They are
// for the other kind of input - a photograph of a screen, which is soft, dim,
// and lit unevenly across the code.
//
// Both variants are local-mean thresholds. ZXing bins the image into fixed
// blocks and thresholds each against its neighbours, which handles a gradient
// across a page but still decides every pixel from one grid; a large local mean
// decides each pixel from its own surroundings, which is what a photograph with
// a bright corner and a dim one needs. The window sizes were measured against
// photographs the plain decode could not read: a window around three eighths of
// the smaller side read them, and much smaller windows started thresholding
// inside single modules.
func normalizedVariants(src image.Image) []image.Image {
	if src == nil {
		return nil
	}
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width < minNormalizedDimension || height < minNormalizedDimension {
		return nil
	}
	if !safeDimensions(width, height) {
		return nil
	}

	gray := grayscale(src)
	smaller := min(width, height)
	variants := make([]image.Image, 0, maxNormalizedVariants)
	variants = append(variants, adaptiveThreshold(gray, thresholdWindow(smaller*3/8), 0))
	variants = append(variants, adaptiveThreshold(unsharpMask(gray), thresholdWindow(smaller/8), 5))
	wipeGray(gray)
	return variants
}

// thresholdWindow keeps the window odd and inside sane bounds. Too small and it
// thresholds within a single module, turning every module into an edge; too
// large and it is the global threshold that failed already.
func thresholdWindow(preferred int) int {
	window := min(max(preferred, 16), 256)
	if window%2 == 0 {
		window++
	}
	return window
}

func grayscale(src image.Image) *image.Gray {
	bounds := src.Bounds()
	out := image.NewGray(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			r, g, b, _ := src.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			// Rec. 601 luma, the same weighting the decoder's own luminance
			// source uses, so the variants differ from it only by the threshold.
			luma := (299*int(r>>8) + 587*int(g>>8) + 114*int(b>>8)) / 1000
			out.SetGray(x, y, color.Gray{Y: uint8(luma)})
		}
	}
	return out
}

func unsharpMask(src *image.Gray) *image.Gray {
	blurred := boxBlur(src, unsharpRadius)
	out := image.NewGray(src.Bounds())
	for index, value := range src.Pix {
		sharpened := float64(value) + unsharpAmount*(float64(value)-float64(blurred.Pix[index]))
		out.Pix[index] = uint8(math.Round(math.Min(255, math.Max(0, sharpened))))
	}
	wipeGray(blurred)
	return out
}

func boxBlur(src *image.Gray, radius int) *image.Gray {
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	out := image.NewGray(bounds)
	sums := integralImage(src)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			total, area := windowSum(sums, width, height, x, y, radius)
			out.SetGray(x, y, color.Gray{Y: uint8(total / area)})
		}
	}
	return out
}

// adaptiveThreshold paints a pixel white when it is brighter than the mean of
// the window around it, less a bias that keeps flat paper from turning to noise.
func adaptiveThreshold(src *image.Gray, window, bias int) *image.Gray {
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	out := image.NewGray(bounds)
	sums := integralImage(src)
	radius := window / 2
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			total, area := windowSum(sums, width, height, x, y, radius)
			if int64(src.GrayAt(x, y).Y) > total/area-int64(bias) {
				out.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
	return out
}

// integralImage lets every window mean cost four lookups instead of a loop, so
// a large window is no more expensive than a small one.
func integralImage(src *image.Gray) []int64 {
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	sums := make([]int64, (width+1)*(height+1))
	for y := 0; y < height; y++ {
		var row int64
		for x := 0; x < width; x++ {
			row += int64(src.GrayAt(x, y).Y)
			sums[(y+1)*(width+1)+x+1] = sums[y*(width+1)+x+1] + row
		}
	}
	return sums
}

func windowSum(sums []int64, width, height, x, y, radius int) (total, area int64) {
	x0, y0 := max(0, x-radius), max(0, y-radius)
	x1, y1 := min(width, x+radius+1), min(height, y+radius+1)
	stride := width + 1
	total = sums[y1*stride+x1] - sums[y0*stride+x1] - sums[y1*stride+x0] + sums[y0*stride+x0]
	return total, int64((x1 - x0) * (y1 - y0))
}

// wipeGray clears a redrawn copy of the code. The frames this package is handed
// are wiped after use, and a variant holds the same image.
func wipeGray(img *image.Gray) {
	if img == nil {
		return
	}
	for index := range img.Pix {
		img.Pix[index] = 0
	}
	runtime.KeepAlive(img)
}

func wipeVariants(variants []image.Image) {
	for _, variant := range variants {
		if gray, ok := variant.(*image.Gray); ok {
			wipeGray(gray)
		}
	}
	runtime.KeepAlive(variants)
}
