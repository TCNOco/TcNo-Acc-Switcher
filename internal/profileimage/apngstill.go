package profileimage

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/draw"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"TcNo-Acc-Switcher/internal/fsutil"
)

// Steam's animated avatar frames are APNGs, and a page cannot freeze one. There
// is no way to pause an animated <img>, and drawImage only ever hands back the
// default image - proven by five captures across three seconds of a running
// animation coming back byte-identical. In these files the default image is
// frame 0, which on a lightning border is the peak of the flash and several
// times thicker than the frame that was on screen a moment before.
//
// So the still is cut here instead, once, when the frame is downloaded. The
// result is an ordinary single-frame PNG: it renders in the same <img>, it looks
// like the animation rather than its loudest moment, and it costs nothing to
// display because there is nothing left to decode.

// ErrNotAnimated means there is nothing to cut a still from, which every caller
// treats as "no work to do" rather than as a failure.
var ErrNotAnimated = errors.New("not an animated PNG")

// AnimatedStillSuffix names the single-frame companion of an animated asset.
const AnimatedStillSuffix = "still"

const pngSignature = "\x89PNG\r\n\x1a\n"

// APNG dispose operations, applied after a frame has been shown.
const (
	disposeNone       = 0
	disposeBackground = 1
	disposePrevious   = 2
)

// APNG blend operations, applied when a frame is drawn.
const (
	blendSource = 0
	blendOver   = 1
)

type pngChunk struct {
	kind    string
	payload []byte
}

type apngFrame struct {
	width, height uint32
	xOff, yOff    uint32
	dispose       byte
	blend         byte
	// data is the frame's compressed image data, already joined across however
	// many IDAT or fdAT chunks carried it.
	data []byte
}

func splitPNGChunks(data []byte) ([]pngChunk, error) {
	if len(data) < len(pngSignature) || string(data[:len(pngSignature)]) != pngSignature {
		return nil, errors.New("not a PNG")
	}
	var chunks []pngChunk
	for offset := len(pngSignature); offset+12 <= len(data); {
		length := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		if length < 0 || offset+12+length > len(data) {
			return nil, errors.New("truncated PNG chunk")
		}
		kind := string(data[offset+4 : offset+8])
		payload := data[offset+8 : offset+8+length]
		chunks = append(chunks, pngChunk{kind: kind, payload: payload})
		offset += 12 + length
		if kind == "IEND" {
			break
		}
	}
	if len(chunks) == 0 {
		return nil, errors.New("no PNG chunks")
	}
	return chunks, nil
}

// parseAPNG pulls out the header, the palette chunks a frame needs to decode,
// and every animation frame in order.
func parseAPNG(chunks []pngChunk) (header []byte, palette []pngChunk, frames []apngFrame, err error) {
	var current *apngFrame
	animated := false
	seenIDAT := false
	// idatIsFrame0 records whether a frame control chunk came before IDAT, which
	// is what makes the default image the animation's first frame rather than a
	// separate still for viewers that cannot play it.
	idatIsFrame0 := false

	for _, chunk := range chunks {
		switch chunk.kind {
		case "IHDR":
			if len(chunk.payload) < 13 {
				return nil, nil, nil, errors.New("short IHDR")
			}
			header = chunk.payload
		case "PLTE", "tRNS":
			palette = append(palette, chunk)
		case "acTL":
			animated = true
		case "fcTL":
			if len(chunk.payload) < 26 {
				return nil, nil, nil, errors.New("short fcTL")
			}
			if !seenIDAT {
				idatIsFrame0 = true
			}
			frames = append(frames, apngFrame{
				width:   binary.BigEndian.Uint32(chunk.payload[4:8]),
				height:  binary.BigEndian.Uint32(chunk.payload[8:12]),
				xOff:    binary.BigEndian.Uint32(chunk.payload[12:16]),
				yOff:    binary.BigEndian.Uint32(chunk.payload[16:20]),
				dispose: chunk.payload[24],
				blend:   chunk.payload[25],
			})
			current = &frames[len(frames)-1]
		case "IDAT":
			seenIDAT = true
			if idatIsFrame0 && current != nil {
				current.data = append(current.data, chunk.payload...)
			}
		case "fdAT":
			if len(chunk.payload) < 4 || current == nil {
				continue
			}
			// The four byte sequence number is APNG's own ordering field and is
			// not part of the image data.
			current.data = append(current.data, chunk.payload[4:]...)
		}
	}
	if !animated || len(frames) == 0 {
		return nil, nil, nil, ErrNotAnimated
	}
	return header, palette, frames, nil
}

func appendChunk(buf *bytes.Buffer, kind string, payload []byte) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(payload)))
	buf.Write(length[:])
	crc := crc32.NewIEEE()
	buf.WriteString(kind)
	crc.Write([]byte(kind))
	buf.Write(payload)
	crc.Write(payload)
	var sum [4]byte
	binary.BigEndian.PutUint32(sum[:], crc.Sum32())
	buf.Write(sum[:])
}

// decodeFrame rebuilds one animation frame as a standalone PNG and decodes it.
//
// A frame's data is an ordinary PNG image stream for its own sub-rectangle; only
// the header saying how big that rectangle is has to be replaced, which is why
// this needs no PNG decoder of its own.
func decodeFrame(header []byte, palette []pngChunk, frame apngFrame) (image.Image, error) {
	if frame.width == 0 || frame.height == 0 || len(frame.data) == 0 {
		return nil, errors.New("empty frame")
	}
	ihdr := make([]byte, len(header))
	copy(ihdr, header)
	binary.BigEndian.PutUint32(ihdr[0:4], frame.width)
	binary.BigEndian.PutUint32(ihdr[4:8], frame.height)

	var buf bytes.Buffer
	buf.WriteString(pngSignature)
	appendChunk(&buf, "IHDR", ihdr)
	for _, chunk := range palette {
		appendChunk(&buf, chunk.kind, chunk.payload)
	}
	appendChunk(&buf, "IDAT", frame.data)
	appendChunk(&buf, "IEND", nil)
	return png.Decode(bytes.NewReader(buf.Bytes()))
}

// coverage is how much of the canvas a composited frame actually paints. It is
// the measure used to pick a representative frame: a lightning border spends
// most of its time thin and briefly flares, so the median frame looks like what
// the eye reads as "the border" and the brightest is the one that looks wrong.
func coverage(img *image.NRGBA) int64 {
	var total int64
	for i := 3; i < len(img.Pix); i += 4 {
		total += int64(img.Pix[i])
	}
	return total
}

// EnsureAnimatedStill writes a single-frame companion beside a cached animated
// asset and returns its public URL, or "" when the asset is not animated and
// therefore already holds still on its own.
//
// Cheap to call on every refresh: it does the work only when the companion is
// missing or older than the asset it was cut from.
func EnsureAnimatedStill(platformKey, accountID string) string {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return ""
	}
	sourcePath, ok := CachedFilePath(platformKey, accountID)
	if !ok || !strings.EqualFold(filepath.Ext(sourcePath), ".png") {
		return ""
	}
	stillID := accountID + AnimatedStillSuffix
	if stillPath, ok := CachedFilePath(platformKey, stillID); ok {
		source, sErr := os.Stat(sourcePath)
		still, tErr := os.Stat(stillPath)
		if sErr == nil && tErr == nil && !still.ModTime().Before(source.ModTime()) {
			return PublicPath(platformKey, stillID, "png")
		}
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return ""
	}
	still, err := StillFromAPNG(data)
	if err != nil {
		if !errors.Is(err, ErrNotAnimated) {
			slog.Debug("animated still could not be cut",
				"platform", platformKey, "accountID", accountID, "err", err)
		}
		return ""
	}
	dir, err := ProfileDir(platformKey)
	if err != nil {
		return ""
	}
	dest := filepath.Join(dir, stillID+".png")
	if err := fsutil.WriteFileAtomic(dest, still, 0o644); err != nil {
		return ""
	}
	return PublicPath(platformKey, stillID, "png")
}

// StillFromAPNG returns a representative single frame of an animated PNG,
// encoded as an ordinary PNG. It reports ErrNotAnimated for anything that is not
// an APNG, which callers treat as "nothing to do" rather than as a failure.
func StillFromAPNG(data []byte) ([]byte, error) {
	chunks, err := splitPNGChunks(data)
	if err != nil {
		return nil, err
	}
	header, palette, frames, err := parseAPNG(chunks)
	if err != nil {
		return nil, err
	}
	width := int(binary.BigEndian.Uint32(header[0:4]))
	height := int(binary.BigEndian.Uint32(header[4:8]))
	if width <= 0 || height <= 0 || width > 4096 || height > 4096 {
		return nil, fmt.Errorf("implausible APNG canvas %dx%d", width, height)
	}

	// Two passes: the first measures every frame, the second stops at whichever
	// one turned out to be the median. Replaying is far cheaper than keeping a
	// composited copy of a hundred and twenty frames.
	metrics, err := composite(header, palette, frames, width, height, -1, nil)
	if err != nil {
		return nil, err
	}
	if len(metrics) == 0 {
		return nil, ErrNotAnimated
	}
	ordered := append([]int64(nil), metrics...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	median := ordered[len(ordered)/2]
	chosen := 0
	for i, m := range metrics {
		if m == median {
			chosen = i
			break
		}
	}

	canvas := image.NewNRGBA(image.Rect(0, 0, width, height))
	if _, err := composite(header, palette, frames, width, height, chosen, canvas); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := png.Encode(&out, canvas); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// composite replays the animation. With stopAt < 0 it runs to the end and
// returns each frame's coverage; otherwise it stops once the given frame has
// been drawn, leaving the result in out.
func composite(
	header []byte, palette []pngChunk, frames []apngFrame,
	width, height, stopAt int, out *image.NRGBA,
) ([]int64, error) {
	canvas := image.NewNRGBA(image.Rect(0, 0, width, height))
	var metrics []int64
	var previous *image.NRGBA

	for index, frame := range frames {
		rect := image.Rect(int(frame.xOff), int(frame.yOff),
			int(frame.xOff+frame.width), int(frame.yOff+frame.height))
		if !rect.In(canvas.Bounds()) {
			return nil, fmt.Errorf("frame %d lies outside the canvas", index)
		}
		if frame.dispose == disposePrevious {
			previous = image.NewNRGBA(canvas.Bounds())
			copy(previous.Pix, canvas.Pix)
		}

		img, err := decodeFrame(header, palette, frame)
		if err != nil {
			return nil, fmt.Errorf("frame %d: %w", index, err)
		}
		op := draw.Over
		if frame.blend == blendSource {
			// Source replaces the rectangle outright, alpha included, which Over
			// cannot express.
			op = draw.Src
		}
		draw.Draw(canvas, rect, img, image.Point{}, op)

		if stopAt < 0 {
			metrics = append(metrics, coverage(canvas))
		} else if index == stopAt {
			copy(out.Pix, canvas.Pix)
			return nil, nil
		}

		switch frame.dispose {
		case disposeBackground:
			draw.Draw(canvas, rect, image.Transparent, image.Point{}, draw.Src)
		case disposePrevious:
			if previous != nil {
				copy(canvas.Pix, previous.Pix)
			}
		case disposeNone:
		}
	}
	if stopAt >= 0 {
		return nil, fmt.Errorf("frame %d not reached", stopAt)
	}
	return metrics, nil
}
