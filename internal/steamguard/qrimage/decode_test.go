package qrimage

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"reflect"
	"testing"
	"time"

	"TcNo-Acc-Switcher/internal/steamguard/qr"

	goqr "github.com/piglig/go-qr"
)

func TestDecodePNGAndWipe(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	source.SetNRGBA(1, 1, color.NRGBA{R: 1, G: 2, B: 3, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}
	frame, err := Decode(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if frame.Width != 4 || frame.Height != 3 || len(frame.Pixels) != 4*3*4 {
		t.Fatalf("frame = %#v", frame)
	}
	pixels := frame.Pixels
	frame.Wipe()
	if frame.Pixels != nil {
		t.Fatal("frame retained pixels")
	}
	for _, value := range pixels {
		if value != 0 {
			t.Fatal("pixel buffer was not wiped")
		}
	}
}

func TestDecodeJPEG(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, source, nil); err != nil {
		t.Fatal(err)
	}
	frame, err := Decode(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer frame.Wipe()
	if frame.Width != 8 || frame.Height != 8 {
		t.Fatalf("size = %dx%d", frame.Width, frame.Height)
	}
}

func TestDecodeCompositesTransparencyOntoWhite(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}
	frame, err := Decode(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer frame.Wipe()
	if got := frame.Image().NRGBAAt(0, 0); got != (color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}) {
		t.Fatalf("transparent pixel = %#v", got)
	}
}

func TestDecodeRejectsUnsafeInputs(t *testing.T) {
	if _, err := Decode(bytes.NewReader([]byte("<svg/>"))); !errors.Is(err, ErrUnsupportedImage) {
		t.Fatalf("SVG error = %v", err)
	}
	if _, err := Decode(bytes.NewReader(make([]byte, MaxEncodedBytes+1))); !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("oversize error = %v", err)
	}
	if _, err := Load("relative.png"); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("relative path error = %v", err)
	}
}

func TestDecodeRejectsOversizeDimensions(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, MaxDimension+1, 1))
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(bytes.NewReader(encoded.Bytes())); !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("dimension error = %v", err)
	}
}

func TestDecodeRejectsAPNGMarker(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}
	data := encoded.Bytes()
	marker := []byte{0, 0, 0, 0, 'a', 'c', 'T', 'L', 0, 0, 0, 0}
	withAnimation := append(append([]byte(nil), data[:8]...), append(marker, data[8:]...)...)
	if _, err := Decode(bytes.NewReader(withAnimation)); !errors.Is(err, ErrAnimatedImage) {
		t.Fatalf("APNG error = %v", err)
	}
}

func TestDecodeCandidatesReturnsCanonicalSteamChallengeAndWipesFrame(t *testing.T) {
	payload := "https://s.team/q/1/1234567890123456789"
	frame := makeQRFrame(t, payload)
	pixels := frame.Pixels

	candidates, err := DecodeCandidates(context.Background(), frame)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Candidate{{Payload: payload}}; !reflect.DeepEqual(candidates, want) {
		t.Fatalf("candidates = %#v, want %#v", candidates, want)
	}
	requireFrameWiped(t, frame, pixels)
}

func TestDecodeCandidatesFindsChallengeInsideScreenshot(t *testing.T) {
	payload := "https://s.team/q/1/42"
	qrFrame := makeQRFrame(t, payload)
	frame := makeOpaqueFrame(qrFrame.Width+240, qrFrame.Height+160, color.NRGBA{
		R: 0xff,
		G: 0xff,
		B: 0xff,
		A: 0xff,
	})
	draw.Draw(frame.Image(), image.Rect(160, 80, 160+qrFrame.Width, 80+qrFrame.Height), qrFrame.Image(), image.Point{}, draw.Src)
	draw.Draw(frame.Image(), image.Rect(20, 20, 120, 50), image.NewUniform(color.Black), image.Point{}, draw.Src)
	qrFrame.Wipe()

	candidates, err := DecodeCandidates(context.Background(), frame)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Candidate{{Payload: payload}}; !reflect.DeepEqual(candidates, want) {
		t.Fatalf("candidates = %#v, want %#v", candidates, want)
	}
}

func TestDecodeCandidatesReturnsUniqueCandidatesAcrossFrames(t *testing.T) {
	first := "https://s.team/q/1/1"
	second := "https://s.team/q/1/18446744073709551615"
	frames := []*Frame{
		makeQRFrame(t, first),
		makeQRFrame(t, first),
		makeQRFrame(t, second),
	}
	buffers := frameBuffers(frames)

	candidates, err := DecodeCandidates(context.Background(), frames...)
	if err != nil {
		t.Fatal(err)
	}
	want := []Candidate{{Payload: first}, {Payload: second}}
	if !reflect.DeepEqual(candidates, want) {
		t.Fatalf("candidates = %#v, want %#v", candidates, want)
	}
	for index := range frames {
		requireFrameWiped(t, frames[index], buffers[index])
	}
}

func TestDecodeCandidatesGeneratedFrameWithNoSymbol(t *testing.T) {
	frame := makeOpaqueFrame(480, 320, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
	pixels := frame.Pixels
	candidates, err := DecodeCandidates(context.Background(), frame)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Fatalf("candidates = %#v", candidates)
	}
	requireFrameWiped(t, frame, pixels)
}

func TestDecodeCandidatesGeneratedFrameWithTwoDistinctSymbols(t *testing.T) {
	first := "https://s.team/q/1/1111111111111111111"
	second := "https://s.team/q/1/2222222222222222222"
	frame := makeQRComposite(t, first, second)
	pixels := frame.Pixels

	candidates, err := DecodeCandidates(context.Background(), frame)
	if err != nil {
		t.Fatal(err)
	}
	want := []Candidate{{Payload: first}, {Payload: second}}
	if !reflect.DeepEqual(candidates, want) {
		t.Fatalf("candidates = %#v, want %#v", candidates, want)
	}
	requireFrameWiped(t, frame, pixels)
}

func TestDecodeCandidatesGeneratedFrameDeduplicatesRepeatedSymbol(t *testing.T) {
	payload := "https://s.team/q/1/3333333333333333333"
	frame := makeQRComposite(t, payload, payload)
	pixels := frame.Pixels

	candidates, err := DecodeCandidates(context.Background(), frame)
	if err != nil {
		t.Fatal(err)
	}
	want := []Candidate{{Payload: payload}}
	if !reflect.DeepEqual(candidates, want) {
		t.Fatalf("candidates = %#v, want %#v", candidates, want)
	}
	requireFrameWiped(t, frame, pixels)
}

func TestDecodeCandidatesCapsDistinctPayloadsAndWipesUnscannedFrames(t *testing.T) {
	frames := []*Frame{
		makeQRFrame(t, "https://s.team/q/1/1"),
		makeQRFrame(t, "https://s.team/q/1/2"),
		makeQRFrame(t, "https://s.team/q/1/3"),
	}
	buffers := frameBuffers(frames)
	candidates, err := DecodeCandidates(context.Background(), frames...)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != MaxUniqueCandidates {
		t.Fatalf("candidate count = %d", len(candidates))
	}
	for index := range frames {
		requireFrameWiped(t, frames[index], buffers[index])
	}
}

func TestDecodeRegionsBoundsCallsAndPixelWork(t *testing.T) {
	frame := makeOpaqueFrame(801, 603, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
	defer frame.Wipe()
	regions, err := decodeRegions(frame.Image())
	if err != nil {
		t.Fatal(err)
	}
	if len(regions) == 0 || len(regions) > MaxDecodeRegions {
		t.Fatalf("region count = %d", len(regions))
	}
	var sampled int64
	for _, region := range regions {
		bounds := region.Bounds()
		sampled += int64(bounds.Dx()) * int64(bounds.Dy())
	}
	maximum := int64(frame.Width) * int64(frame.Height) * MaxDecodePixelMultiplier
	if sampled > maximum {
		t.Fatalf("sampled pixels = %d, maximum = %d", sampled, maximum)
	}
}

func TestDecodeCandidatesFiltersNonSteamAndNonCanonicalPayloads(t *testing.T) {
	frames := []*Frame{
		makeQRFrame(t, "https://example.com/q/1/1"),
		makeQRFrame(t, "https://S.TEAM/q/1/1"),
		makeQRFrame(t, "https://s.team/q/1/01"),
		makeQRFrame(t, "https://s.team/q/2/1"),
	}

	candidates, err := DecodeCandidates(context.Background(), frames...)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Fatalf("candidates = %#v", candidates)
	}
}

func TestDecodeCandidatesValidatesWholeBatchBeforeDecode(t *testing.T) {
	valid := makeQRFrame(t, "https://s.team/q/1/1")
	invalid := &Frame{Width: 21, Height: 21, Stride: 83, Pixels: make([]byte, 83*21)}
	validPixels := valid.Pixels
	invalidPixels := invalid.Pixels

	if _, err := DecodeCandidates(context.Background(), valid, invalid); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("error = %v", err)
	}
	requireFrameWiped(t, valid, validPixels)
	requireFrameWiped(t, invalid, invalidPixels)
}

func TestDecodeCandidatesBoundsFramesAndAggregatePixels(t *testing.T) {
	frames := make([]*Frame, MaxCandidateFrames+1)
	for index := range frames {
		frames[index] = makeBlankFrame(21, 21)
	}
	if _, err := DecodeCandidates(context.Background(), frames...); !errors.Is(err, ErrTooManyFrames) {
		t.Fatalf("frame count error = %v", err)
	}

	frames = []*Frame{makeBlankFrame(21, 21), makeBlankFrame(21, 21)}
	if err := validateCandidateFrames(frames, 2, 21*21); !errors.Is(err, ErrDecodeWorkLimit) {
		t.Fatalf("aggregate pixel error = %v", err)
	}
	wipeFrames(frames)
}

func TestDecodeCandidatesHonorsCancellationAndWipesFrames(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	frame := makeBlankFrame(21, 21)
	pixels := frame.Pixels

	if _, err := DecodeCandidates(ctx, frame); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	requireFrameWiped(t, frame, pixels)
}

func TestDecodeCandidatesDiscardsResultWhenCanceledDuringDecode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	frame := makeBlankFrame(21, 21)
	pixels := frame.Pixels
	decoder := func(image.Image) (string, error) {
		cancel()
		return "https://s.team/q/1/1", nil
	}

	candidates, err := decodeCandidates(ctx, []*Frame{frame}, decoder, time.Now, 1, 21*21, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("candidates = %#v", candidates)
	}
	requireFrameWiped(t, frame, pixels)
}

func TestDecodeCandidatesEnforcesElapsedTimeBudget(t *testing.T) {
	started := time.Unix(0, 0)
	expired := false
	now := func() time.Time {
		if expired {
			return started.Add(time.Second)
		}
		return started
	}
	decoder := func(image.Image) (string, error) {
		expired = true
		return "https://s.team/q/1/1", nil
	}
	frame := makeBlankFrame(21, 21)

	if _, err := decodeCandidates(
		context.Background(),
		[]*Frame{frame},
		decoder,
		now,
		1,
		21*21,
		time.Second,
	); !errors.Is(err, ErrDecodeTimeout) {
		t.Fatalf("error = %v", err)
	}
}

func TestDecodeFrameRecoversDecoderPanics(t *testing.T) {
	if _, err := decodeFrame(panickingImage{}); !errors.Is(err, ErrDecoderFailure) {
		t.Fatalf("error = %v", err)
	}
}

func FuzzDecode(f *testing.F) {
	f.Add([]byte("not an image"))
	f.Add([]byte("\x89PNG\r\n\x1a\n"))
	f.Fuzz(func(t *testing.T, input []byte) {
		frame, _ := Decode(bytes.NewReader(input))
		if frame != nil {
			frame.Wipe()
		}
	})
}

func FuzzDecodeCandidatesPayload(f *testing.F) {
	f.Add("https://s.team/q/1/123456789")
	f.Add("https://evil.example/q/1/1")
	f.Add("\x00https://s.team/q/1/1")
	f.Fuzz(func(t *testing.T, payload string) {
		frame := makeBlankFrame(1, 1)
		decoder := func(image.Image) (string, error) { return payload, nil }
		candidates, err := decodeCandidates(
			context.Background(),
			[]*Frame{frame},
			decoder,
			time.Now,
			1,
			1,
			time.Second,
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(candidates) > 1 {
			t.Fatalf("candidate count = %d", len(candidates))
		}
		if len(candidates) == 1 {
			if _, err := qr.ParseChallenge(payload); err != nil {
				t.Fatalf("accepted invalid payload %q: %v", payload, err)
			}
			if candidates[0].Payload != payload {
				t.Fatalf("candidate = %q, want %q", candidates[0].Payload, payload)
			}
		}
	})
}

func FuzzDecodeCandidatesFrameShape(f *testing.F) {
	f.Add(1, 1, 4, []byte{0, 0, 0, 0xff})
	f.Add(21, 21, 83, []byte("bad stride"))
	f.Fuzz(func(t *testing.T, width, height, stride int, input []byte) {
		pixels := append([]byte(nil), input...)
		frame := &Frame{Width: width, Height: height, Stride: stride, Pixels: pixels}
		decoder := func(image.Image) (string, error) { return "", errors.New("no QR") }
		_, _ = decodeCandidates(
			context.Background(),
			[]*Frame{frame},
			decoder,
			time.Now,
			1,
			MaxCandidatePixels,
			time.Second,
		)
		for _, value := range pixels {
			if value != 0 {
				t.Fatal("pixel buffer was not wiped")
			}
		}
	})
}

func FuzzDecodeCandidatePixels(f *testing.F) {
	f.Add([]byte{0, 0, 0, 0xff})
	f.Add([]byte("finder-like-1:1:3:1:1"))
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) == 0 {
			return
		}
		width := 21 + int(input[0]%76)
		height := width
		frame := makeBlankFrame(width, height)
		for index := range frame.Pixels {
			frame.Pixels[index] = input[index%len(input)]
		}
		_, _ = DecodeCandidates(context.Background(), frame)
		if frame.Pixels != nil || frame.Width != 0 || frame.Height != 0 || frame.Stride != 0 {
			t.Fatal("frame was not wiped")
		}
	})
}

func FuzzDecodeCandidatesImages(f *testing.F) {
	seedFrames := []*Frame{
		makeOpaqueFrame(480, 320, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}),
		makeQRComposite(f, "https://s.team/q/1/1"),
		makeQRComposite(f, "https://s.team/q/1/1", "https://s.team/q/1/2"),
		makeQRComposite(f, "https://s.team/q/1/1", "https://s.team/q/1/1"),
	}
	for _, frame := range seedFrames {
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, frame.Image()); err != nil {
			f.Fatal(err)
		}
		f.Add(append([]byte(nil), encoded.Bytes()...))
		frame.Wipe()
	}

	f.Fuzz(func(t *testing.T, encoded []byte) {
		frame, err := Decode(bytes.NewReader(encoded))
		if err != nil {
			return
		}
		candidates, _ := DecodeCandidates(context.Background(), frame)
		if len(candidates) > MaxUniqueCandidates {
			t.Fatalf("candidate count = %d", len(candidates))
		}
	})
}

func makeQRFrame(t testing.TB, payload string) *Frame {
	t.Helper()
	symbol, err := goqr.EncodeText(payload, goqr.Medium)
	if err != nil {
		t.Fatal(err)
	}
	source, err := symbol.ToImage(goqr.NewQrCodeImgConfig(6, 4))
	if err != nil {
		t.Fatal(err)
	}
	bounds := source.Bounds()
	frame := &Frame{
		Width:  bounds.Dx(),
		Height: bounds.Dy(),
		Stride: bounds.Dx() * 4,
		Pixels: make([]byte, bounds.Dx()*bounds.Dy()*4),
	}
	draw.Draw(frame.Image(), frame.Image().Bounds(), source, bounds.Min, draw.Src)
	return frame
}

func makeQRComposite(t testing.TB, payloads ...string) *Frame {
	t.Helper()
	const (
		padding = 32
		gap     = 64
	)
	if len(payloads) == 0 {
		return makeOpaqueFrame(480, 320, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
	}
	symbols := make([]*Frame, len(payloads))
	width := padding * 2
	height := 0
	for index, payload := range payloads {
		symbols[index] = makeQRFrame(t, payload)
		width += symbols[index].Width
		if index > 0 {
			width += gap
		}
		if symbols[index].Height > height {
			height = symbols[index].Height
		}
	}
	height += padding * 2
	frame := makeOpaqueFrame(width, height, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
	x := padding
	for _, symbol := range symbols {
		y := (height - symbol.Height) / 2
		draw.Draw(frame.Image(), image.Rect(x, y, x+symbol.Width, y+symbol.Height), symbol.Image(), image.Point{}, draw.Src)
		x += symbol.Width + gap
		symbol.Wipe()
	}
	return frame
}

func makeBlankFrame(width, height int) *Frame {
	return &Frame{
		Width:  width,
		Height: height,
		Stride: width * 4,
		Pixels: make([]byte, width*height*4),
	}
}

func makeOpaqueFrame(width, height int, fill color.NRGBA) *Frame {
	frame := makeBlankFrame(width, height)
	draw.Draw(frame.Image(), frame.Image().Bounds(), image.NewUniform(fill), image.Point{}, draw.Src)
	return frame
}

func frameBuffers(frames []*Frame) [][]byte {
	buffers := make([][]byte, len(frames))
	for index := range frames {
		buffers[index] = frames[index].Pixels
	}
	return buffers
}

func requireFrameWiped(t *testing.T, frame *Frame, pixels []byte) {
	t.Helper()
	if frame.Width != 0 || frame.Height != 0 || frame.Stride != 0 || frame.Pixels != nil {
		t.Fatalf("frame retained data: %#v", frame)
	}
	for _, value := range pixels {
		if value != 0 {
			t.Fatal("pixel buffer was not wiped")
		}
	}
}

type panickingImage struct{}

func (panickingImage) ColorModel() color.Model { return color.NRGBAModel }
func (panickingImage) Bounds() image.Rectangle { panic("untrusted image") }
func (panickingImage) At(int, int) color.Color { return color.NRGBA{} }
