package qrimage

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

const (
	MaxEncodedBytes = 8 << 20
	MaxDimension    = 8192
	MaxPixels       = 24_000_000
)

var (
	ErrUnsupportedImage = errors.New("unsupported QR screenshot format")
	ErrImageTooLarge    = errors.New("QR screenshot exceeds safe limits")
	ErrAnimatedImage    = errors.New("animated QR screenshots are not supported")
	ErrInvalidImage     = errors.New("invalid QR screenshot")
	ErrUnsafePath       = errors.New("unsafe QR screenshot path")
)

type Frame struct {
	Width  int
	Height int
	Stride int
	Pixels []byte
}

func (f *Frame) Image() *image.NRGBA {
	if f == nil {
		return nil
	}
	return &image.NRGBA{Pix: f.Pixels, Stride: f.Stride, Rect: image.Rect(0, 0, f.Width, f.Height)}
}

func (f *Frame) Wipe() {
	if f == nil {
		return
	}
	pixels := f.Pixels
	clear(pixels)
	runtime.KeepAlive(pixels)
	f.Pixels = nil
	f.Width = 0
	f.Height = 0
	f.Stride = 0
}

func Load(path string) (*Frame, error) {
	if !filepath.IsAbs(path) {
		return nil, ErrUnsafePath
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, errors.Join(ErrUnsafePath, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrUnsafePath
	}
	if info.Size() <= 0 || info.Size() > MaxEncodedBytes {
		return nil, ErrImageTooLarge
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.Join(ErrUnsafePath, err)
	}
	defer file.Close()
	return Decode(file)
}

func Decode(reader io.Reader) (*Frame, error) {
	if reader == nil {
		return nil, ErrInvalidImage
	}
	encoded, err := io.ReadAll(io.LimitReader(reader, MaxEncodedBytes+1))
	if err != nil {
		return nil, ErrInvalidImage
	}
	defer func() {
		clear(encoded)
		runtime.KeepAlive(encoded)
	}()
	if len(encoded) == 0 || len(encoded) > MaxEncodedBytes {
		return nil, ErrImageTooLarge
	}
	format, err := validateContainer(encoded)
	if err != nil {
		return nil, err
	}
	config, configFormat, err := image.DecodeConfig(bytes.NewReader(encoded))
	if err != nil || configFormat != format || !safeDimensions(config.Width, config.Height) {
		if err == nil && configFormat == format {
			return nil, ErrImageTooLarge
		}
		return nil, ErrInvalidImage
	}
	decoded, decodedFormat, err := image.Decode(bytes.NewReader(encoded))
	if err != nil || decodedFormat != format {
		return nil, ErrInvalidImage
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != config.Width || bounds.Dy() != config.Height || !safeDimensions(bounds.Dx(), bounds.Dy()) {
		return nil, ErrInvalidImage
	}
	frame := &Frame{
		Width:  bounds.Dx(),
		Height: bounds.Dy(),
		Stride: bounds.Dx() * 4,
		Pixels: make([]byte, bounds.Dx()*bounds.Dy()*4),
	}
	destination := frame.Image()
	draw.Draw(destination, destination.Bounds(), image.NewUniform(color.NRGBA{
		R: 0xff,
		G: 0xff,
		B: 0xff,
		A: 0xff,
	}), image.Point{}, draw.Src)
	draw.Draw(destination, destination.Bounds(), decoded, bounds.Min, draw.Over)
	return frame, nil
}

func safeDimensions(width, height int) bool {
	if width <= 0 || height <= 0 || width > MaxDimension || height > MaxDimension {
		return false
	}
	return int64(width)*int64(height) <= MaxPixels
}

func validateContainer(data []byte) (string, error) {
	if len(data) >= 8 && bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")) {
		if err := validatePNGChunks(data); err != nil {
			return "", err
		}
		return "png", nil
	}
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "jpeg", nil
	}
	return "", ErrUnsupportedImage
}

func validatePNGChunks(data []byte) error {
	position := 8
	seenEnd := false
	for position < len(data) {
		if len(data)-position < 12 {
			return ErrInvalidImage
		}
		length := int64(binary.BigEndian.Uint32(data[position : position+4]))
		chunkEnd := int64(position) + 12 + length
		if length < 0 || chunkEnd > int64(len(data)) {
			return ErrInvalidImage
		}
		chunkType := string(data[position+4 : position+8])
		if chunkType == "acTL" {
			return ErrAnimatedImage
		}
		position = int(chunkEnd)
		if chunkType == "IEND" {
			seenEnd = true
			break
		}
	}
	if !seenEnd || position != len(data) {
		return ErrInvalidImage
	}
	return nil
}
