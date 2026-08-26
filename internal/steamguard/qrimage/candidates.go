package qrimage

import (
	"context"
	"errors"
	"image"
	"image/color"
	"runtime"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/qr"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

const (
	// MaxCandidateFrames bounds the number of independently normalized frames
	// accepted by one decode attempt.
	MaxCandidateFrames = 8
	// MaxCandidatePixels bounds total work across a decode attempt, not just one
	// frame.
	MaxCandidatePixels = MaxPixels
	// MaxCandidateDecodeTime is the elapsed-time budget checked around each
	// synchronous decoder call.
	MaxCandidateDecodeTime = 3 * time.Second
	// MaxUniqueCandidates bounds retained QR payloads. Two distinct canonical
	// challenges are sufficient to reject an ambiguous frame, so retaining more
	// would only keep additional sensitive strings alive.
	MaxUniqueCandidates = 2
	// MaxDecodeRegions bounds decoder calls per frame. The pinned decoder returns
	// only its first symbol, so a full-frame pass is followed by overlapping
	// horizontal and vertical partitions.
	MaxDecodeRegions = 5
	// MaxDecodePixelMultiplier bounds aggregate pixels sampled across all regions
	// of a frame. The current five-region layout samples less than 3.5x the frame.
	MaxDecodePixelMultiplier = 4
)

var (
	ErrTooManyFrames   = errors.New("too many QR frames")
	ErrDecodeWorkLimit = errors.New("QR decode work limit exceeded")
	ErrDecodeTimeout   = errors.New("QR decode time limit exceeded")
	ErrDecoderFailure  = errors.New("QR decoder failed")
)

// Candidate is a canonical Steam mobile-login QR payload. A candidate is
// untrusted until a higher layer binds it to an authenticated account and
// explicitly authorizes it.
type Candidate struct {
	Payload string
}

type frameDecoder func(image.Image) (string, error)
type decodeClock func() time.Time

// DecodeCandidates consumes normalized frames and returns up to two unique
// canonical Steam QR payloads in first-seen order. Two means "multiple"; the
// caller must reject rather than choose one. Every supplied frame is wiped
// before return, including on validation, cancellation, or decoder failure.
//
// The underlying pure-Go decoder cannot be interrupted mid-call. Strict frame
// and aggregate pixel limits cap that work; context and elapsed-time failures
// are observed immediately before and after each call.
func DecodeCandidates(ctx context.Context, frames ...*Frame) ([]Candidate, error) {
	return decodeCandidates(
		ctx,
		frames,
		decodeFrame,
		time.Now,
		MaxCandidateFrames,
		MaxCandidatePixels,
		MaxCandidateDecodeTime,
	)
}

func decodeCandidates(
	ctx context.Context,
	frames []*Frame,
	decoder frameDecoder,
	now decodeClock,
	maxFrames int,
	maxPixels int64,
	maxDuration time.Duration,
) ([]Candidate, error) {
	defer wipeFrames(frames)

	if ctx == nil || decoder == nil || now == nil || maxFrames <= 0 || maxPixels <= 0 || maxDuration <= 0 {
		return nil, ErrInvalidImage
	}
	if err := validateCandidateFrames(frames, maxFrames, maxPixels); err != nil {
		return nil, err
	}

	started := now()
	if err := checkDecodeBudget(ctx, started, now, maxDuration); err != nil {
		return nil, err
	}

	candidates := make([]Candidate, 0, MaxUniqueCandidates)
	seen := make(map[string]struct{}, MaxUniqueCandidates)
	for _, frame := range frames {
		regions, err := decodeRegions(frame.Image())
		if err != nil {
			return nil, err
		}
		for _, region := range regions {
			if err := checkDecodeBudget(ctx, started, now, maxDuration); err != nil {
				return nil, err
			}

			payload, decodeErr := decoder(region)
			if errors.Is(decodeErr, ErrDecoderFailure) {
				return nil, ErrDecoderFailure
			}
			if budgetErr := checkDecodeBudget(ctx, started, now, maxDuration); budgetErr != nil {
				return nil, budgetErr
			}
			if decodeErr != nil {
				continue
			}
			if _, parseErr := qr.ParseChallenge(payload); parseErr != nil {
				continue
			}
			if _, duplicate := seen[payload]; duplicate {
				continue
			}
			seen[payload] = struct{}{}
			candidates = append(candidates, Candidate{Payload: payload})
			if len(candidates) == MaxUniqueCandidates {
				return candidates, nil
			}
		}
		// A screenshot decodes on the first pass and never pays for this; a
		// photograph of a screen is the input that needs redrawing to be read.
		if len(candidates) == 0 {
			found, err := decodeVariants(ctx, frame, decoder, now, started, maxDuration)
			if err != nil {
				return nil, err
			}
			if found != "" {
				candidates = append(candidates, Candidate{Payload: found})
			}
		}
		frame.Wipe()
	}
	if err := checkDecodeBudget(ctx, started, now, maxDuration); err != nil {
		return nil, err
	}
	return candidates, nil
}

// decodeVariants reads re-drawn copies of a frame whole rather than in regions:
// splitting the frame was not what stopped the first attempt.
func decodeVariants(
	ctx context.Context,
	frame *Frame,
	decoder frameDecoder,
	now decodeClock,
	started time.Time,
	maxDuration time.Duration,
) (string, error) {
	variants := normalizedVariants(frame.Image())
	defer wipeVariants(variants)
	for _, variant := range variants {
		if err := checkDecodeBudget(ctx, started, now, maxDuration); err != nil {
			return "", err
		}
		payload, decodeErr := decoder(variant)
		if errors.Is(decodeErr, ErrDecoderFailure) {
			return "", ErrDecoderFailure
		}
		if budgetErr := checkDecodeBudget(ctx, started, now, maxDuration); budgetErr != nil {
			return "", budgetErr
		}
		if decodeErr != nil {
			continue
		}
		if _, parseErr := qr.ParseChallenge(payload); parseErr != nil {
			continue
		}
		return payload, nil
	}
	return "", nil
}

// decodeRegions returns views over the source image; it never copies pixels.
// Panics from an adversarial image implementation are contained as decoder
// failures. The caller owns and wipes the source frame after all views expire.
func decodeRegions(source image.Image) (regions []image.Image, err error) {
	defer func() {
		if recover() != nil {
			regions = nil
			err = ErrDecoderFailure
		}
	}()
	if source == nil {
		return nil, ErrInvalidImage
	}
	bounds := source.Bounds()
	if !safeDimensions(bounds.Dx(), bounds.Dy()) {
		return nil, ErrInvalidImage
	}

	regions = make([]image.Image, 0, MaxDecodeRegions)
	seen := make(map[image.Rectangle]struct{}, MaxDecodeRegions)
	budget := int64(bounds.Dx()) * int64(bounds.Dy()) * MaxDecodePixelMultiplier
	var sampled int64
	appendRegion := func(rect image.Rectangle) {
		rect = rect.Intersect(bounds)
		if rect.Dx() < 21 || rect.Dy() < 21 {
			return
		}
		if _, duplicate := seen[rect]; duplicate {
			return
		}
		pixels := int64(rect.Dx()) * int64(rect.Dy())
		if len(regions) >= MaxDecodeRegions || sampled+pixels > budget {
			return
		}
		seen[rect] = struct{}{}
		sampled += pixels
		if rect == bounds {
			regions = append(regions, source)
			return
		}
		if subImager, ok := source.(interface {
			SubImage(image.Rectangle) image.Image
		}); ok {
			regions = append(regions, subImager.SubImage(rect))
			return
		}
		regions = append(regions, imageView{source: source, bounds: rect})
	}

	appendRegion(bounds)
	// The overlap is one quarter of each dimension. Two separate QR symbols fit
	// individually in at least one partition without relying on whichever symbol
	// the full-frame decoder happens to choose.
	xLow := bounds.Min.X + bounds.Dx()*3/8
	xHigh := bounds.Min.X + bounds.Dx()*5/8
	yLow := bounds.Min.Y + bounds.Dy()*3/8
	yHigh := bounds.Min.Y + bounds.Dy()*5/8
	appendRegion(image.Rect(bounds.Min.X, bounds.Min.Y, xHigh, bounds.Max.Y))
	appendRegion(image.Rect(xLow, bounds.Min.Y, bounds.Max.X, bounds.Max.Y))
	appendRegion(image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Max.X, yHigh))
	appendRegion(image.Rect(bounds.Min.X, yLow, bounds.Max.X, bounds.Max.Y))
	return regions, nil
}

type imageView struct {
	source image.Image
	bounds image.Rectangle
}

func (v imageView) ColorModel() color.Model { return v.source.ColorModel() }
func (v imageView) Bounds() image.Rectangle { return v.bounds }
func (v imageView) At(x, y int) color.Color { return v.source.At(x, y) }

func validateCandidateFrames(frames []*Frame, maxFrames int, maxPixels int64) error {
	if len(frames) > maxFrames {
		return ErrTooManyFrames
	}
	var totalPixels int64
	for _, frame := range frames {
		if frame == nil || !safeDimensions(frame.Width, frame.Height) {
			return ErrInvalidImage
		}
		if frame.Stride != frame.Width*4 || len(frame.Pixels) != frame.Stride*frame.Height {
			return ErrInvalidImage
		}
		totalPixels += int64(frame.Width) * int64(frame.Height)
		if totalPixels > maxPixels {
			return ErrDecodeWorkLimit
		}
	}
	return nil
}

func checkDecodeBudget(ctx context.Context, started time.Time, now decodeClock, maxDuration time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	current := now()
	if deadline, ok := ctx.Deadline(); ok && !current.Before(deadline) {
		return context.DeadlineExceeded
	}
	if !current.Before(started.Add(maxDuration)) {
		return ErrDecodeTimeout
	}
	return nil
}

// decodeFrame reads the first symbol in one region. A fresh reader per call:
// the decoder holds per-decode state, and two windows can decode at once.
func decodeFrame(frame image.Image) (payload string, err error) {
	defer func() {
		if recover() != nil {
			payload = ""
			err = ErrDecoderFailure
		}
	}()
	bitmap, bitmapErr := gozxing.NewBinaryBitmapFromImage(frame)
	if bitmapErr != nil {
		// Nothing about the image is decodable, which is the same answer as a
		// region with no code in it.
		return "", bitmapErr
	}
	result, decodeErr := qrcode.NewQRCodeReader().Decode(bitmap, nil)
	if decodeErr != nil {
		return "", decodeErr
	}
	return result.GetText(), nil
}

func wipeFrames(frames []*Frame) {
	for _, frame := range frames {
		frame.Wipe()
	}
	runtime.KeepAlive(frames)
}
